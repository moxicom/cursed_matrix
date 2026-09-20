import { useCallback, useEffect, useMemo, useState } from 'react';

import { useBoardStore } from '@/features/board/board.store';
import * as activityApi from '@/shared/api/activity';
import {
  matchesFilters,
  useFiltersStore,
  type DeadlineFilter,
  type TopologyFilter,
} from '@/features/filters/filters.store';
import { TagFilter } from '@/features/filters/TagFilter';
import { useForceGraph } from '@/features/graph/useForceGraph';
import { QUADRANTS, QUADRANT_BY_ID, TASK_COLORS } from '@/shared/config/domain';
import { useLang, useLocalized, useT } from '@/shared/i18n';
import { deadlineInfo } from '@/shared/lib/deadline';
import { effectiveQuadrant } from '@/shared/lib/progression';
import type { Quadrant, Task } from '@/shared/types/domain';
import { Button, Chip, ColorSwatch, Select } from '@/shared/ui';
import { GraphSidebar, GraphSidebarHeader, GraphSidebarSection } from '@/widgets';

export interface GraphPageProps {
  onOpenTask: (id: string) => void;
}

/**
 * Notes that the graph was looked at today.
 *
 * Recorded at most once per day server-side, and outside the heatmap: the
 * only thing that asks is the exploration achievement, which counts the days
 * a user went looking (CLAUDE.md §69).
 */
function useGraphVisit() {
  useEffect(() => {
    void activityApi.graphOpened();
  }, []);
}

export function GraphPage({ onOpenTask }: GraphPageProps) {
  const t = useT();
  const lang = useLang();
  const localized = useLocalized();
  // Selected as the stored array, mapped here: a selector that built the
  // list would hand back a new one on every render, and a store that
  // compares by reference would call that a change — for ever.
  useGraphVisit();

  const tags = useBoardStore((s) => s.tags);
  const tagStats = useMemo(
    () => tags.map((tag) => ({ name: tag.name, count: tag.taskCount })),
    [tags],
  );
  const tasks = useBoardStore((s) => s.tasks);
  const links = useBoardStore((s) => s.links);
  const loaded = useBoardStore((s) => s.loaded);
  const taskById = useBoardStore((s) => s.taskById);
  const subtasksOf = useBoardStore((s) => s.subtasksOf);
  const filters = useFiltersStore();
  const [hoverId, setHoverId] = useState<string | null>(null);

  const quadrantOf = useCallback(
    (task: Task): Quadrant | null =>
      effectiveQuadrant(
        task,
        task.parentTaskId === null ? undefined : taskById(task.parentTaskId),
      ),
    [taskById],
  );

  const criteria = useMemo(
    () => ({
      status: filters.status,
      tags: filters.tags,
      colors: filters.colors,
      quadrants: filters.quadrants,
      deadline: filters.deadline,
      topology: filters.topology,
      query: filters.query,
    }),
    [
      filters.status,
      filters.tags,
      filters.colors,
      filters.quadrants,
      filters.deadline,
      filters.topology,
      filters.query,
    ],
  );

  const { nodes, edges, linkCountById } = useMemo(() => {
    const counts = new Map<string, number>();
    links.forEach((link) => {
      counts.set(link.sourceTaskId, (counts.get(link.sourceTaskId) ?? 0) + 1);
      counts.set(link.targetTaskId, (counts.get(link.targetTaskId) ?? 0) + 1);
    });

    const visible = tasks.filter((task) =>
      matchesFilters(task, criteria, {
        quadrant: quadrantOf(task),
        linkCount: counts.get(task.id) ?? 0,
      }),
    );
    const visibleIds = new Set(visible.map((task) => task.id));

    return {
      nodes: visible,
      edges: links.filter(
        (link) => visibleIds.has(link.sourceTaskId) && visibleIds.has(link.targetTaskId),
      ),
      linkCountById: counts,
    };
    // quadrantOf reads taskById, which is stable in the store
  }, [tasks, links, criteria, quadrantOf]);

  const { attachCanvas, zoom, zoomBy, fit } = useForceGraph({
    nodes,
    links: edges,
    quadrantOf,
    linkCountOf: (id) => linkCountById.get(id) ?? 0,
    subtaskCountOf: (id) => subtasksOf(id).length,
    selectedId: null,
    onSelect: onOpenTask,
    onHover: setHoverId,
  });

  const hovered = hoverId === null ? undefined : taskById(hoverId);
  const hoveredQuadrant = hovered === undefined ? null : quadrantOf(hovered);
  const hoveredMeta = hoveredQuadrant === null ? null : QUADRANT_BY_ID[hoveredQuadrant];
  const topologyOptions: Array<{ value: TopologyFilter; label: string }> = [
    { value: 'any', label: t.any },
    { value: 'linked', label: t.linked },
    { value: 'unlinked', label: t.unlinked },
  ];

  const legend = [
    ...QUADRANTS.map((q) => ({ fill: '#1c2226', stroke: q.accent, opacity: 1, label: q.code })),
    {
      fill: '#191d20',
      stroke: '#2b3236',
      opacity: 0.4,
      label: lang === 'RU' ? 'завершённый узел' : 'completed node',
    },
    {
      fill: '#4f93ad',
      stroke: '#4f93ad',
      opacity: 1,
      label: lang === 'RU' ? 'цвет задачи' : 'user task color',
    },
    {
      fill: '#0a0b0c',
      stroke: '#8b9599',
      opacity: 1,
      label: lang === 'RU' ? 'подзадача (полый)' : 'subtask (hollow)',
    },
  ];

  return (
    <div className="flex min-h-0 flex-1">
      <GraphSidebar>
        <GraphSidebarHeader
          title={t.netFilters}
          stats={
            loaded
              ? `${nodes.length}${lang === 'RU' ? ' узлов · ' : ' nodes · '}${edges.length}${
                  lang === 'RU' ? ' связей' : ' edges'
                }`
              : t.working
          }
        />

        <GraphSidebarSection label={t.state}>
          <div className="flex flex-wrap gap-4">
            {(['active', 'completed', 'all'] as const).map((value) => (
              <Chip
                key={value}
                size="xs"
                active={filters.status === value}
                onClick={() => filters.setStatus(value)}
              >
                {value === 'active' ? t.active : value === 'completed' ? t.completed : t.all}
              </Chip>
            ))}
          </div>
        </GraphSidebarSection>

        <GraphSidebarSection label={t.quadrant}>
          <div className="flex flex-col gap-3">
            {QUADRANTS.map((meta) => (
              <Chip
                key={meta.id}
                tone="cyan"
                size="xs"
                className="justify-start text-left"
                active={filters.quadrants.includes(meta.id)}
                onClick={() => filters.toggleQuadrant(meta.id)}
              >
                <span className="font-bold" style={{ color: meta.accent }}>
                  {meta.code}
                </span>
                <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">
                  {localized(meta.subtitle)}
                </span>
                <span className="text-txt-faint">
                  {tasks.filter((task) => task.quadrant === meta.id).length}
                </span>
              </Chip>
            ))}
          </div>
        </GraphSidebarSection>

        <GraphSidebarSection label={t.tags}>
          <TagFilter tags={tagStats} layout="wrap" chipSize="xs" />
        </GraphSidebarSection>

        <GraphSidebarSection label={t.color}>
          <div className="flex gap-5">
            {TASK_COLORS.map((color) => (
              <ColorSwatch
                key={color.id}
                size="md"
                color={color.hex}
                title={localized(color.name)}
                selected={filters.colors.includes(color.id)}
                onClick={() => filters.toggleColor(color.id)}
              />
            ))}
          </div>
        </GraphSidebarSection>

        <GraphSidebarSection label={t.topology}>
          <div className="flex flex-wrap gap-4">
            {topologyOptions.map((option) => (
              <Chip
                key={option.value}
                tone="cyan"
                size="xs"
                active={filters.topology === option.value}
                onClick={() => filters.setTopology(option.value)}
              >
                {option.label}
              </Chip>
            ))}
          </div>
        </GraphSidebarSection>

        <GraphSidebarSection label={t.deadline}>
          <Select
            className="h-26 w-full"
            value={filters.deadline}
            onChange={(event) => filters.setDeadline(event.target.value as DeadlineFilter)}
            options={[
              { value: 'any', label: t.dlAny },
              { value: 'overdue', label: t.dlOverdue },
              { value: 'today', label: t.dlToday },
              { value: 'week', label: t.dlWeek },
              { value: 'none', label: t.dlNone },
            ]}
          />
        </GraphSidebarSection>

        <GraphSidebarSection label={t.legend} last>
          <div className="flex flex-col gap-6">
            {legend.map((item) => (
              <div key={item.label} className="flex items-center gap-8 text-95 text-txt-dim">
                <span
                  className="h-10 w-10 flex-none rounded-full border"
                  style={{ background: item.fill, borderColor: item.stroke, opacity: item.opacity }}
                />
                {item.label}
              </div>
            ))}
          </div>
          <Button block variant="danger" size="xs" className="mt-12" onClick={filters.reset}>
            {t.reset}
          </Button>
        </GraphSidebarSection>
      </GraphSidebar>

      <div className="relative min-w-0 flex-1 bg-bg-base">
        <canvas ref={attachCanvas} className="absolute inset-0 block h-full w-full cursor-grab" />

        <div className="pointer-events-none absolute left-14 top-12 text-95 tracking-t9 text-txt-ghost">
          {t.netMap} :: {t.dragHint}
        </div>

        <div className="absolute right-14 top-10 flex gap-4">
          <Button variant="solid" size="xs" className="h-26 w-26" onClick={() => zoomBy(1.25)}>
            +
          </Button>
          <Button variant="solid" size="xs" className="h-26 w-26" onClick={() => zoomBy(0.8)}>
            −
          </Button>
          <Button variant="solid" size="xs" className="h-26 w-26" onClick={fit}>
            ⊙
          </Button>
        </div>

        {hovered !== undefined && (
          <div className="pointer-events-none absolute bottom-14 left-14 max-w-360 border border-line-strong bg-bg-panel px-11 py-9">
            <div className="flex items-center gap-9 text-9 tracking-t5">
              <span className="text-txt-ghost">{hovered.id.toUpperCase()}</span>
              <span
                className="font-bold"
                style={{ color: hoveredMeta?.accent ?? '#7d878c' }}
              >
                {hoveredMeta?.code ?? 'SUB'}
              </span>
              <span className={hovered.status === 'COMPLETED' ? 'text-green' : 'text-txt-dim'}>
                [{hovered.status === 'COMPLETED' ? t.completed : hovered.parentTaskId ? 'SUB' : t.active}]
              </span>
            </div>
            <div className="mt-5 text-11 leading-[1.4] text-txt">{hovered.title}</div>
            <div className="mt-5 text-95 text-txt-dim">
              {[
                deadlineInfo(hovered, t.completed).text,
                hovered.tags.map((tag) => `#${tag}`).join(' '),
                `◈${linkCountById.get(hovered.id) ?? 0}`,
              ]
                .filter((part) => part !== '')
                .join('   ')}
            </div>
          </div>
        )}

        <div className="absolute bottom-14 right-14 text-95 text-txt-ghost">
          ZOOM {zoom.toFixed(2)}x
        </div>
      </div>
    </div>
  );
}
