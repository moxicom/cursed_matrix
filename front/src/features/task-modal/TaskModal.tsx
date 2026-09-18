import { useState } from 'react';

import { useBoardStore } from '@/features/board/board.store';
import { LINK_TYPES, QUADRANT_BY_ID, QUADRANTS, TASK_COLORS } from '@/shared/config/domain';
import {
  atLimit,
  counterFor,
  DESCRIPTION_MAX_LENGTH,
  TAG_MAX_LENGTH,
  TITLE_MAX_LENGTH,
} from '@/shared/config/limits';
import { useLocalized, useT } from '@/shared/i18n';
import { cn } from '@/shared/lib/cn';
import { deadlineInfo, DEADLINE_TONE_CLASS } from '@/shared/lib/deadline';
import { xpForTask } from '@/shared/lib/progression';
import type { LinkType, Quadrant, TaskColorId } from '@/shared/types/domain';
import {
  Badge,
  Button,
  Checkbox,
  Chip,
  ColorSwatch,
  FieldLabel,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Select,
  TagChip,
  Textarea,
} from '@/shared/ui';

export interface TaskModalProps {
  taskId: string | null;
  onClose: () => void;
  onOpenTask: (id: string) => void;
}

export function TaskModal({ taskId, onClose, onOpenTask }: TaskModalProps) {
  const t = useT();
  const localized = useLocalized();
  const store = useBoardStore();
  const [newSub, setNewSub] = useState('');
  const [newTag, setNewTag] = useState('');

  const task = taskId === null ? undefined : store.taskById(taskId);
  if (!task) return null;

  const isSub = task.parentTaskId !== null;
  const parent = isSub ? store.taskById(task.parentTaskId as string) : undefined;
  const quadrant: Quadrant = task.quadrant ?? parent?.quadrant ?? 'IMPORTANT_URGENT';
  const meta = QUADRANT_BY_ID[quadrant];
  const done = task.status === 'COMPLETED';
  const subs = store.subtasksOf(task.id);
  const links = store.linksOf(task.id);
  const dl = deadlineInfo(task, t.completed);

  const deadlineDate = task.deadlineAt === null ? '' : task.deadlineAt.slice(0, 10);
  const deadlineTime = task.deadlineAt === null ? '' : new Date(task.deadlineAt).toTimeString().slice(0, 5);

  const linkCandidates = store.tasks
    .filter(
      (candidate) =>
        candidate.id !== task.id &&
        !links.some(
          (l) => l.sourceTaskId === candidate.id || l.targetTaskId === candidate.id,
        ),
    )
    .slice(0, 12)
    .map((candidate) => ({
      value: candidate.id,
      label: `${
        candidate.quadrant === null ? 'SUB' : QUADRANT_BY_ID[candidate.quadrant].code
      } · ${candidate.title}`,
    }));

  const setDeadline = (date: string, time: string) => {
    if (date === '') {
      store.patchTask(task.id, { deadlineAt: null, deadlineHasTime: false });
      return;
    }
    const iso = new Date(`${date}T${time === '' ? '23:59' : time}`).toISOString();
    store.patchTask(task.id, { deadlineAt: iso, deadlineHasTime: time !== '' });
  };

  return (
    <Modal open onClose={onClose} size="lg" layer="modal">
      <ModalHeader>
        <span className="text-10 font-bold tracking-t7" style={{ color: meta.accent }}>
          {meta.code}
        </span>
        <span className="text-95 text-txt-ghost">{task.id.toUpperCase()}</span>
        <Badge tone={done ? 'green' : 'neutral'}>{done ? t.completed : t.active}</Badge>
        {isSub && parent !== undefined && (
          <span className="text-95 text-txt-dim">
            {t.parentNode}: {parent.title}
          </span>
        )}
        <span className="flex-1" />
        <span className="text-95 text-violet-dim">
          {done ? `${t.xpLocked} ` : '+'}
          {done ? (task.xpAwarded ?? 0) : xpForTask(quadrant, isSub)} XP
        </span>
        <IconButton size="md" tone="danger" onClick={onClose}>
          ×
        </IconButton>
      </ModalHeader>

      <ModalBody className="grid grid-cols-1 gap-px bg-line-subtle md:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <div className="flex flex-col gap-14 bg-bg-raised p-14">
          <div>
            <FieldLabel
              className="mb-5"
              {...(counterFor(task.title, TITLE_MAX_LENGTH) !== undefined
                ? { aside: counterFor(task.title, TITLE_MAX_LENGTH) }
                : {})}
            >
              {t.title}
            </FieldLabel>
            <Input
              inputSize="lg"
              maxLength={TITLE_MAX_LENGTH}
              value={task.title}
              className={cn(atLimit(task.title, TITLE_MAX_LENGTH) && 'border-red focus:border-red')}
              onChange={(event) => store.patchTask(task.id, { title: event.target.value })}
            />
          </div>

          <div>
            <FieldLabel
              className="mb-5"
              {...(counterFor(task.description, DESCRIPTION_MAX_LENGTH) !== undefined
                ? { aside: counterFor(task.description, DESCRIPTION_MAX_LENGTH) }
                : {})}
            >
              {t.description}
            </FieldLabel>
            <Textarea
              value={task.description}
              maxLength={DESCRIPTION_MAX_LENGTH}
              placeholder={t.descPh}
              className={cn(
                atLimit(task.description, DESCRIPTION_MAX_LENGTH) && 'border-red focus:border-red',
              )}
              onChange={(event) => store.patchTask(task.id, { description: event.target.value })}
            />
          </div>

          <div>
            <FieldLabel
              className="mb-6"
              aside={`${subs.filter((s) => s.status === 'COMPLETED').length} / ${subs.length} ${t.done}`}
            >
              {t.subtasks}
            </FieldLabel>

            {isSub ? (
              <div className="border border-line bg-bg-input px-11 py-9 text-10 leading-[1.5] text-txt-dim">
                {t.subNoNest}
              </div>
            ) : (
              <div className="flex flex-col gap-3">
                {subs.map((sub) => (
                  <div
                    key={sub.id}
                    className="flex items-center gap-6 border border-line-subtle bg-bg-input px-7 py-5"
                    style={{ borderLeft: '2px solid #2e343a' }}
                  >
                    <Checkbox
                      size="sm"
                      checked={sub.status === 'COMPLETED'}
                      onChange={() => store.toggleDone(sub.id)}
                    />
                    <Input
                      variant="bare"
                      inputSize="sm"
                      maxLength={TITLE_MAX_LENGTH}
                      className={cn(
                        'flex-1 px-0 py-0 text-11',
                        sub.status === 'COMPLETED' && 'text-txt-tag line-through',
                        atLimit(sub.title, TITLE_MAX_LENGTH) && 'text-red',
                      )}
                      value={sub.title}
                      onChange={(event) => store.patchTask(sub.id, { title: event.target.value })}
                    />
                    <span className="whitespace-nowrap text-9 text-violet-sub">
                      {sub.status === 'COMPLETED' ? '' : '+'}
                      {sub.status === 'COMPLETED'
                        ? (sub.xpAwarded ?? 0)
                        : xpForTask(quadrant, true)}{' '}
                      XP
                    </span>
                    <Button variant="ghost" size="xs" onClick={() => store.promoteSubtask(sub.id)}>
                      {t.promote}
                    </Button>
                    <IconButton onClick={() => onOpenTask(sub.id)} title={t.openTask}>
                      →
                    </IconButton>
                  </div>
                ))}

                <div className="mt-3 flex gap-5">
                  <Input
                    variant="dashed"
                    inputSize="sm"
                    maxLength={TITLE_MAX_LENGTH}
                    placeholder={t.newSub}
                    className={cn(atLimit(newSub, TITLE_MAX_LENGTH) && 'border-red focus:border-red')}
                    value={newSub}
                    onChange={(event) => setNewSub(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' && newSub.trim() !== '') {
                        store.createSubtask(task.id, newSub.trim());
                        setNewSub('');
                      }
                    }}
                  />
                  <Button
                    variant="solid"
                    className="text-green"
                    onClick={() => {
                      if (newSub.trim() === '') return;
                      store.createSubtask(task.id, newSub.trim());
                      setNewSub('');
                    }}
                  >
                    + {t.add}
                  </Button>
                </div>
              </div>
            )}
          </div>

          <div>
            <FieldLabel className="mb-6" aside={`◈ ${links.length}`}>
              {t.linkedNodes}
            </FieldLabel>
            <div className="flex flex-col gap-3">
              {links.map((link) => {
                const otherId = link.sourceTaskId === task.id ? link.targetTaskId : link.sourceTaskId;
                const other = store.taskById(otherId);
                if (!other) return null;
                const otherQuad = other.quadrant ?? 'IMPORTANT_URGENT';
                return (
                  <div
                    key={link.id}
                    className="flex items-center gap-8 border border-line-subtle bg-bg-input px-9 py-6"
                  >
                    <span className="text-10 text-cyan">◈</span>
                    <span
                      className="text-9 font-bold"
                      style={{ color: QUADRANT_BY_ID[otherQuad].accent }}
                    >
                      {QUADRANT_BY_ID[otherQuad].code}
                    </span>
                    <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-105 text-txt-soft">
                      {other.title}
                    </span>
                    <Select
                      className="h-22 text-9"
                      value={link.type}
                      options={LINK_TYPES.map((type) => ({ value: type, label: type }))}
                      onChange={(event) => store.setLinkType(link.id, event.target.value as LinkType)}
                    />
                    <IconButton onClick={() => onOpenTask(other.id)}>→</IconButton>
                    <IconButton tone="danger" onClick={() => store.removeLink(link.id)}>
                      ×
                    </IconButton>
                  </div>
                );
              })}

              <Select
                variant="dashed"
                className="mt-3 h-30"
                value=""
                placeholder={t.addLinkPh}
                options={linkCandidates}
                onChange={(event) => {
                  if (event.target.value !== '') store.addLink(task.id, event.target.value);
                }}
              />
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-14 bg-bg-alt p-14">
          <div>
            <FieldLabel className="mb-6">{t.priority}</FieldLabel>
            <div className="flex flex-col gap-3">
              {QUADRANTS.map((option) => (
                <Chip
                  key={option.id}
                  tone="cyan"
                  active={quadrant === option.id}
                  className="justify-start text-left"
                  onClick={() => {
                    if (isSub) return;
                    store.moveTask(task.id, option.id, null);
                  }}
                >
                  <span className="flex-none font-bold" style={{ color: option.accent }}>
                    {option.code}
                  </span>
                  <span className="min-w-0 flex-1 break-words">{localized(option.subtitle)}</span>
                  <span className="flex-none whitespace-nowrap text-violet-dim">
                    +{xpForTask(option.id, isSub)} XP
                  </span>
                </Chip>
              ))}
            </div>
            <div className="mt-6 text-9 leading-[1.5] text-txt-faint">{t.xpNote}</div>
          </div>

          <div>
            <FieldLabel className="mb-6">{t.deadline}</FieldLabel>
            <div className="flex gap-5">
              <Input
                type="date"
                inputSize="sm"
                value={deadlineDate}
                onChange={(event) => setDeadline(event.target.value, deadlineTime)}
              />
              <Input
                type="time"
                inputSize="sm"
                className="w-82"
                value={deadlineTime}
                onChange={(event) => setDeadline(deadlineDate, event.target.value)}
              />
            </div>
            <div className="mt-6 flex items-center gap-8">
              <span className={cn('border px-6 py-2 text-95 tracking-t5', DEADLINE_TONE_CLASS[dl.tone])}>
                {dl.text}
              </span>
              <Button
                variant="danger"
                size="xs"
                onClick={() => store.patchTask(task.id, { deadlineAt: null, deadlineHasTime: false })}
              >
                {t.clear}
              </Button>
            </div>
          </div>

          <div>
            <FieldLabel className="mb-6">{t.taskColor}</FieldLabel>
            <div className="flex flex-wrap gap-5">
              {TASK_COLORS.map((option) => (
                <ColorSwatch
                  key={option.id}
                  size="lg"
                  color={option.hex}
                  title={localized(option.name)}
                  selected={task.color === option.id}
                  onClick={() => store.setTaskColor(task.id, option.id as TaskColorId)}
                />
              ))}
            </div>
            <div className="mt-6 text-9 leading-[1.5] text-txt-faint">{t.colorNote}</div>
          </div>

          <div>
            <FieldLabel
              className="mb-6"
              {...(counterFor(newTag, TAG_MAX_LENGTH) !== undefined
                ? { aside: counterFor(newTag, TAG_MAX_LENGTH) }
                : {})}
            >
              {t.tags}
            </FieldLabel>
            <div className="flex flex-wrap gap-4">
              {task.tags.map((tag) => (
                <TagChip key={tag} name={tag} onRemove={() => store.removeTag(task.id, tag)} />
              ))}
            </div>
            <div className="mt-6 flex gap-5">
              <Input
                variant="dashed"
                inputSize="sm"
                maxLength={TAG_MAX_LENGTH}
                placeholder={t.newTagPh}
                className={cn(atLimit(newTag, TAG_MAX_LENGTH) && 'border-red focus:border-red')}
                value={newTag}
                onChange={(event) => setNewTag(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && newTag.trim() !== '') {
                    store.addTag(task.id, newTag.trim());
                    setNewTag('');
                  }
                }}
              />
              <Button
                variant="solid"
                className="text-cyan"
                onClick={() => {
                  if (newTag.trim() === '') return;
                  store.addTag(task.id, newTag.trim());
                  setNewTag('');
                }}
              >
                +
              </Button>
            </div>
          </div>

          <div className="flex flex-col gap-5 border-t border-line-subtle pt-11">
            {[
              { k: t.nodeId, v: task.id.toUpperCase() },
              { k: t.created, v: task.createdAt.slice(0, 10) },
              { k: t.completedAt, v: task.completedAt?.slice(0, 10) ?? '—' },
              { k: t.position, v: String(task.position) },
            ].map((row) => (
              <div key={row.k} className="flex items-baseline gap-8 text-95">
                <span className="w-96 flex-none text-txt-label">{row.k}</span>
                <span className="min-w-0 flex-1 break-words text-txt-dim">{row.v}</span>
              </div>
            ))}
          </div>
        </div>
      </ModalBody>

      <ModalFooter>
        <Button
          variant={done ? 'ghost' : 'primary'}
          size="lg"
          onClick={() => store.toggleDone(task.id)}
        >
          {done ? t.reopen : t.resolve}
        </Button>
        {isSub && (
          <Button variant="ghost" size="md" className="text-cyan" onClick={() => store.promoteSubtask(task.id)}>
            {t.promoteSelf}
          </Button>
        )}
        <span className="flex-1" />
        <span className="text-9 text-txt-label">{t.autosave}</span>
        <Button
          variant="danger"
          size="md"
          onClick={() => {
            store.deleteTask(task.id);
            onClose();
          }}
        >
          {t.delete}
        </Button>
        <Button variant="solid" size="md" onClick={onClose}>
          {t.close}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
