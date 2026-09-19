package httphandler

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/tag"
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
