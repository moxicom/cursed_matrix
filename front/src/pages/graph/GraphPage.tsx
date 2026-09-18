import { useState } from 'react';

import { useBoardStore } from '@/features/board/board.store';
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
import { mockTagStats } from '@/shared/mocks';
import type { Quadrant, Task } from '@/shared/types/domain';
import { Button, Chip, ColorSwatch, Select } from '@/shared/ui';
import { GraphSidebar, GraphSidebarHeader, GraphSidebarSection } from '@/widgets';

export interface GraphPageProps {
  onOpenTask: (id: string) => void;
}

export function GraphPage({ onOpenTask }: GraphPageProps) {
  const t = useT();
  const lang = useLang();
  const localized = useLocalized();
  const store = useBoardStore();
  const filters = useFiltersStore();
  const [hoverId, setHoverId] = useState<string | null>(null);

  const quadrantOf = (task: Task): Quadrant => {
    if (task.quadrant !== null) return task.quadrant;
    const parent = task.parentTaskId === null ? undefined : store.taskById(task.parentTaskId);
    return parent?.quadrant ?? 'IMPORTANT_URGENT';
  };

  const criteria = {
    status: filters.status,
    tags: filters.tags,
    colors: filters.colors,
    quadrants: filters.quadrants,
    deadline: filters.deadline,
    topology: filters.topology,
    query: filters.query,
  };

  const nodes = store.tasks.filter((task) =>
    matchesFilters(task, criteria, {
      quadrant: quadrantOf(task),
      linkCount: store.linksOf(task.id).length,
    }),
  );
  const visibleIds = new Set(nodes.map((task) => task.id));
  const edges = store.links.filter(
    (link) => visibleIds.has(link.sourceTaskId) && visibleIds.has(link.targetTaskId),
  );

  const graph = useForceGraph({
    nodes,
    links: edges,
    quadrantOf,
    linkCountOf: (id) => store.linksOf(id).length,
    subtaskCountOf: (id) => store.subtasksOf(id).length,
    selectedId: null,
    onSelect: onOpenTask,
    onHover: setHoverId,
  });

  const hovered = hoverId === null ? undefined : store.taskById(hoverId);
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
          stats={`${nodes.length}${lang === 'RU' ? ' узлов · ' : ' nodes · '}${edges.length}${
            lang === 'RU' ? ' связей' : ' edges'
          }`}
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
                  {store.tasks.filter((task) => task.quadrant === meta.id).length}
                </span>
              </Chip>
            ))}
          </div>
        </GraphSidebarSection>

        <GraphSidebarSection label={t.tags}>
          <TagFilter tags={mockTagStats} layout="wrap" chipSize="xs" />
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
        <canvas ref={graph.canvasRef} className="absolute inset-0 block h-full w-full cursor-grab" />

        <div className="pointer-events-none absolute left-14 top-12 text-95 tracking-t9 text-txt-ghost">
          {t.netMap} :: {t.dragHint}
        </div>

        <div className="absolute right-14 top-10 flex gap-4">
          <Button variant="solid" size="xs" className="h-26 w-26" onClick={() => graph.zoomBy(1.25)}>
            +
          </Button>
          <Button variant="solid" size="xs" className="h-26 w-26" onClick={() => graph.zoomBy(0.8)}>
            −
          </Button>
          <Button variant="solid" size="xs" className="h-26 w-26" onClick={graph.fit}>
            ⊙
          </Button>
        </div>

        {hovered !== undefined && (
          <div className="pointer-events-none absolute bottom-14 left-14 max-w-360 border border-line-strong bg-bg-panel px-11 py-9">
            <div className="flex items-center gap-9 text-9 tracking-t5">
              <span className="text-txt-ghost">{hovered.id.toUpperCase()}</span>
              <span
                className="font-bold"
                style={{ color: QUADRANT_BY_ID[quadrantOf(hovered)].accent }}
              >
                {QUADRANT_BY_ID[quadrantOf(hovered)].code}
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
                `◈${store.linksOf(hovered.id).length}`,
              ]
                .filter((part) => part !== '')
                .join('   ')}
            </div>
          </div>
        )}

        <div className="absolute bottom-14 right-14 text-95 text-txt-ghost">
          ZOOM {graph.zoom.toFixed(2)}x
        </div>
      </div>
    </div>
  );
}
