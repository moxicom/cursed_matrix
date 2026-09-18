import { useState } from 'react';

import {
  AsciiBar,
  Badge,
  Button,
  Checkbox,
  Chip,
  ColorSwatch,
  EmptyState,
  FieldLabel,
  IconButton,
  Identicon,
  Input,
  KeyHint,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Panel,
  SegmentedControl,
  Select,
  StatCard,
  TagChip,
  Textarea,
  Toast,
  Toggle,
} from '@/shared/ui';

const SWATCHES = ['#4f93ad', '#8f7fc4', '#c8973c', '#c05b6a', '#3f9e8f', '#6b7780'];

/** Temporary preview of shared/ui — replaced by real screens in step 5. */
export function UiKitPage() {
  const [status, setStatus] = useState<'active' | 'completed' | 'all'>('active');
  const [period, setPeriod] = useState<'week' | 'month' | 'all'>('all');
  const [tags, setTags] = useState<string[]>(['backend']);
  const [color, setColor] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [visible, setVisible] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [toast, setToast] = useState(false);

  const toggleTag = (tag: string) =>
    setTags((prev) => (prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]));

  return (
    <div className="mx-auto flex max-w-activity flex-col gap-18 px-22 pb-48 pt-20">
      <Panel title="BUTTONS" aside="variant × size">
        <div className="flex flex-wrap items-center gap-8 p-14">
          <Button variant="primary" size="lg">
            <span>&gt;</span>ENTER_TERMINAL
          </Button>
          <Button variant="violet" size="md">
            ACTIVATE_SUBSCRIPTION
          </Button>
          <Button variant="solid">CLOSE</Button>
          <Button variant="ghost">RESET</Button>
          <Button variant="danger">DELETE</Button>
          <Button variant="dashed" size="md">
            + new task
          </Button>
          <Button variant="plain" size="xs">
            PRICING
          </Button>
          <Button variant="ghost" disabled>
            DISABLED
          </Button>
        </div>
      </Panel>

      <Panel title="CHIPS · TOGGLES · SWATCHES">
        <div className="flex flex-col gap-14 p-14">
          <div className="flex flex-wrap items-center gap-12">
            <FieldLabel>STATE</FieldLabel>
            <SegmentedControl
              options={[
                { value: 'active', label: 'ACTIVE' },
                { value: 'completed', label: 'COMPLETED' },
                { value: 'all', label: 'ALL' },
              ]}
              value={status}
              onChange={setStatus}
            />
          </div>
          <div className="flex flex-wrap items-center gap-12">
            <FieldLabel>PERIOD</FieldLabel>
            <SegmentedControl
              tone="cyan"
              options={[
                { value: 'week', label: 'WEEK' },
                { value: 'month', label: 'MONTH' },
                { value: 'all', label: 'ALL TIME' },
              ]}
              value={period}
              onChange={setPeriod}
            />
          </div>
          <div className="flex flex-wrap items-center gap-12">
            <FieldLabel>TAGS</FieldLabel>
            {['backend', 'design', 'work', 'project-x'].map((tag) => (
              <Chip key={tag} tone="cyan" active={tags.includes(tag)} onClick={() => toggleTag(tag)}>
                #{tag}
              </Chip>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-12">
            <FieldLabel>COLOR</FieldLabel>
            <div className="flex gap-5">
              <ColorSwatch color={null} selected={color === null} onClick={() => setColor(null)} size="lg" />
              {SWATCHES.map((hex) => (
                <ColorSwatch
                  key={hex}
                  color={hex}
                  size="lg"
                  selected={color === hex}
                  onClick={() => setColor(hex)}
                />
              ))}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-12">
            <FieldLabel>PRIVACY</FieldLabel>
            <Toggle checked={visible} onChange={setVisible} labelOn="ENABLED" labelOff="DISABLED" />
            <Checkbox checked={done} onChange={setDone} />
            <span className="text-11 text-txt-dim">terminal checkbox</span>
          </div>
        </div>
      </Panel>

      <Panel title="INPUTS">
        <div className="grid gap-14 p-14 md:grid-cols-2">
          <div>
            <FieldLabel className="mb-5">TITLE</FieldLabel>
            <Input inputSize="lg" defaultValue="Patch auth token refresh race condition" />
          </div>
          <div>
            <FieldLabel className="mb-5">NEW TAG</FieldLabel>
            <Input variant="dashed" inputSize="sm" placeholder="tag name" />
          </div>
          <div className="md:col-span-2">
            <FieldLabel className="mb-5">DESCRIPTION</FieldLabel>
            <Textarea placeholder="context, links, acceptance criteria…" />
          </div>
          <div>
            <FieldLabel className="mb-5">DEADLINE</FieldLabel>
            <Select
              className="w-full"
              options={[
                { value: 'any', label: 'ANY' },
                { value: 'overdue', label: 'OVERDUE' },
                { value: 'today', label: 'TODAY' },
                { value: 'week', label: 'NEXT 7 DAYS' },
                { value: 'none', label: 'NO DEADLINE' },
              ]}
              defaultValue="any"
            />
          </div>
          <div>
            <FieldLabel className="mb-5">LINK</FieldLabel>
            <Select
              className="w-full"
              variant="dashed"
              placeholder="+ link another task…"
              options={[
                { value: 't02', label: 'Q1 · Investor update deck' },
                { value: 't07', label: 'Q2 · Design system motion tokens' },
              ]}
              defaultValue=""
            />
          </div>
        </div>
      </Panel>

      <Panel title="BADGES · TAGS · BARS · IDENTICON">
        <div className="flex flex-col gap-14 p-14">
          <div className="flex flex-wrap items-center gap-8">
            <Badge tone="green" bracketed>
              ACTIVE
            </Badge>
            <Badge tone="amber" bracketed>
              COMING_SOON
            </Badge>
            <Badge tone="violet">LVL.12</Badge>
            <Badge tone="red">! OVERDUE 26H</Badge>
            <Badge tone="cyan">◈ 3</Badge>
            <Badge>SEP 18 14:30</Badge>
          </div>
          <div className="flex flex-wrap items-center gap-8">
            <TagChip name="backend" onRemove={() => undefined} />
            <TagChip name="project-x" onRemove={() => undefined} />
            <TagChip name="design" plain />
          </div>
          <div className="flex flex-wrap items-center gap-12">
            <AsciiBar ratio={0.62} size={8} />
            <AsciiBar ratio={0.62} size={16} className="text-15" />
            <span className="text-105 text-txt-dim">2870/4140 XP</span>
          </div>
          <div className="flex flex-wrap items-center gap-14">
            <Identicon name="nullptr_ok" cell={3} />
            <Identicon name="nullptr_ok" cell={13} />
            <Identicon name="k.orlov" cell={13} accent="#2e343a" />
            <KeyHint>CTRL+K</KeyHint>
            <KeyHint bordered>ESC</KeyHint>
            <IconButton tone="green">+</IconButton>
            <IconButton size="md" tone="danger">
              ×
            </IconButton>
            <IconButton size="lg">⊙</IconButton>
          </div>
        </div>
      </Panel>

      <div className="grid grid-cols-[repeat(auto-fit,minmax(168px,1fr))] gap-px border border-line bg-line-subtle">
        <StatCard label="COMPLETED · 365D" value={312} valueColor="#5fb37f" note="from activity log" />
        <StatCard label="CREATED · 365D" value={418} note="new nodes" />
        <StatCard label="LOGIN STREAK" value="18D" valueColor="#d2a04a" note="best 31D" />
        <StatCard label="LIFETIME XP" value="7 420" valueColor="#9b8fd0" note="LVL.12" />
      </div>

      <Panel title="EMPTY STATE · OVERLAYS">
        <div className="flex flex-col gap-14 p-14">
          <EmptyState
            art={'┌──────────┐\n│          │\n└──────────┘'}
            message="NO_CRITICAL_QUESTS"
            actionLabel="+ new task"
            onAction={() => undefined}
          />
          <div className="flex flex-wrap gap-8">
            <Button variant="solid" onClick={() => setModalOpen(true)}>
              OPEN MODAL
            </Button>
            <Button variant="ghost" onClick={() => setToast((v) => !v)}>
              TOGGLE TOAST
            </Button>
          </div>
        </div>
      </Panel>

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} size="lg">
        <ModalHeader>
          <span className="text-10 font-bold tracking-t7 text-quad-q1">Q1</span>
          <span className="text-95 text-txt-ghost">T01</span>
          <Badge tone="neutral">ACTIVE</Badge>
          <span className="flex-1" />
          <span className="text-95 text-violet-dim">+50 XP</span>
          <IconButton size="md" tone="danger" onClick={() => setModalOpen(false)}>
            ×
          </IconButton>
        </ModalHeader>
        <ModalBody className="p-14">
          <p className="m-0 text-11 leading-[1.6] text-txt-body">
            Modal shell: header / body / footer, backdrop click and Escape close it, layers match the
            design (search 40, task 50, paywall 60).
          </p>
        </ModalBody>
        <ModalFooter>
          <Button variant="primary" size="md">
            RESOLVE QUEST
          </Button>
          <span className="flex-1" />
          <span className="text-9 text-txt-label">changes saved automatically</span>
          <Button variant="solid" onClick={() => setModalOpen(false)}>
            CLOSE
          </Button>
        </ModalFooter>
      </Modal>

      {toast && <Toast code="QUEST_RESOLVED" detail="+50 XP · Fix billing webhook" />}
    </div>
  );
}
