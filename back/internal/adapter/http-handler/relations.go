package httphandler

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/tag"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

type tagsResponse struct {
	Tags []gen.Tag `json:"tags"`
}

// ListTags answers with the labels the filter offers.
func (a *API) ListTags(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	found, err := a.board.Tags(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	rendered := make([]gen.Tag, 0, len(found))
	for i := range found {
		rendered = append(rendered, renderTag(&found[i]))
	}
	WriteJSON(w, r, http.StatusOK, tagsResponse{Tags: rendered})
}

// AttachTag puts a label on a task.
func (a *API) AttachTag(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.AttachTagJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	attached, err := a.board.AttachTag(r.Context(), userID, taskID, body.Name)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderTag(attached))
}

// DetachTag takes a label off a task.
func (a *API) DetachTag(w http.ResponseWriter, r *http.Request, taskID, tagID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	if err := a.board.DetachTag(r.Context(), userID, taskID, tagID); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateLink joins two tasks.
func (a *API) CreateLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.CreateLinkJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	var linkType shared.LinkType
	if body.Type != nil {
		linkType = shared.LinkType(*body.Type)
	}

	created, err := a.graph.Create(r.Context(), userID, body.SourceTaskId, body.TargetTaskId, linkType)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, renderLink(created))
}

// UpdateLink changes what a link means.
func (a *API) UpdateLink(w http.ResponseWriter, r *http.Request, linkID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.UpdateLinkJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	updated, err := a.graph.Retype(r.Context(), userID, linkID, shared.LinkType(body.Type))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderLink(updated))
}

// DeleteLink breaks a link.
func (a *API) DeleteLink(w http.ResponseWriter, r *http.Request, linkID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	if err := a.graph.Delete(r.Context(), userID, linkID); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renderTag(item *tag.Tag) gen.Tag {
	return gen.Tag{Id: item.ID, Name: item.Name, TaskCount: item.TaskCount}
}

func renderLink(item *link.Link) gen.TaskLink {
	return gen.TaskLink{
		Id:           item.ID,
		SourceTaskId: item.SourceID,
		TargetTaskId: item.TargetID,
		Type:         gen.LinkType(item.Type),
		CreatedAt:    item.CreatedAt,
	}
}

type graphResponse struct {
	Nodes []gen.GraphNode `json:"nodes"`
	Edges []gen.GraphEdge `json:"edges"`
}

// GetGraph answers with the snapshot the canvas renders.
func (a *API) GetGraph(w http.ResponseWriter, r *http.Request, params gen.GetGraphParams) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	filter, err := a.board.GraphFilter(r.Context(), userID, graphQuery(params))
	if err != nil {
		WriteError(w, r, err)
		return
	}

	snapshot, err := a.graph.Snapshot(r.Context(), userID, filter)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := graphResponse{
		Nodes: make([]gen.GraphNode, 0, len(snapshot.Nodes)),
		Edges: make([]gen.GraphEdge, 0, len(snapshot.Edges)),
	}
	for i := range snapshot.Nodes {
		response.Nodes = append(response.Nodes, renderNode(&snapshot.Nodes[i]))
	}
	for i := range snapshot.Edges {
		edge := &snapshot.Edges[i]
		rendered := gen.GraphEdge{
			Id:     edge.ID,
			Source: edge.SourceID,
			Target: edge.TargetID,
			Kind:   gen.GraphEdgeKind(edge.Kind),
		}
		if edge.Type != "" {
			linkType := gen.LinkType(edge.Type)
			rendered.Type = &linkType
		}
		response.Edges = append(response.Edges, rendered)
	}
	WriteJSON(w, r, http.StatusOK, response)
}

func renderNode(node *task.Node) gen.GraphNode {
	rendered := gen.GraphNode{
		Id:           node.ID,
		Title:        node.Title,
		Status:       gen.TaskStatus(node.Status),
		IsSubtask:    node.IsSubtask,
		ParentTaskId: node.ParentID,
		DeadlineAt:   node.DeadlineAt,
		Tags:         node.Tags,
		LinkCount:    node.LinkCount,
		SubtaskCount: node.SubtaskCount,
	}
	if rendered.Tags == nil {
		rendered.Tags = []string{}
	}
	if node.Color != "" {
		color := gen.TaskColor(node.Color)
		rendered.Color = &color
	}
	if node.Quadrant != "" {
		quadrant := gen.Quadrant(node.Quadrant)
		rendered.Quadrant = &quadrant
	}
	return rendered
}

// graphQuery maps the canvas's filters onto the board's, which are the same
// set minus the ordering the graph has no use for.
func graphQuery(params gen.GetGraphParams) board.Query {
	return boardQuery(gen.ListTasksParams{
		Status:    params.Status,
		Quadrants: params.Quadrants,
		Colors:    params.Colors,
		Tags:      params.Tags,
		Deadline:  params.Deadline,
		Topology:  params.Topology,
		Query:     params.Query,
	})
}

type searchResponse struct {
	Items []gen.SearchHit `json:"items"`
}

// Search answers the command palette.
func (a *API) Search(w http.ResponseWriter, r *http.Request, params gen.SearchParams) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	limit := 0
	if params.Limit != nil {
		limit = *params.Limit
	}

	hits, err := a.board.Search(r.Context(), userID, params.Query, limit)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := searchResponse{Items: make([]gen.SearchHit, 0, len(hits))}
	for i := range hits {
		hit := &hits[i]
		rendered := gen.SearchHit{
			Id:           hit.ID,
			Title:        hit.Title,
			IsSubtask:    hit.IsSubtask,
			Status:       gen.TaskStatus(hit.Status),
			MatchedField: gen.SearchHitMatchedField(hit.MatchedField),
			MatchedText:  hit.MatchedText,
		}
		if hit.Quadrant != "" {
			quadrant := gen.Quadrant(hit.Quadrant)
			rendered.Quadrant = &quadrant
		}
		response.Items = append(response.Items, rendered)
	}
	WriteJSON(w, r, http.StatusOK, response)
}
