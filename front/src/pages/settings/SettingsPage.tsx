import type { ReactNode } from 'react';

import { useBoardStore } from '@/features/board/board.store';
import { usePreferencesStore } from '@/features/settings/preferences.store';
import { useSessionStore } from '@/features/session/session.store';
import { FREE_TASK_CAP } from '@/shared/config/domain';
import { useI18nStore, useLang, useT } from '@/shared/i18n';
import { Button, Chip } from '@/shared/ui';
import { PageContainer } from '@/widgets';

interface SettingsRowProps {
  label: string;
  note?: string;
  value?: string;
  children?: ReactNode;
}

function SettingsRow({ label, note, value, children }: SettingsRowProps) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-16 border-b border-line-faint px-14 py-12">
      <div className="min-w-190 flex-1">
        <div className="text-105 text-txt">{label}</div>
        {note !== undefined && (
          <div className="mt-4 text-95 leading-[1.45] text-txt-faint">{note}</div>
        )}
      </div>
      <div className="flex flex-wrap items-center justify-end gap-4">
        {children}
        {value !== undefined && <span className="text-105 text-txt-dim">{value}</span>}
      </div>
    </div>
  );
}

function SettingsGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mb-16 border border-line bg-bg-panel">
      <div className="border-b border-line-subtle px-14 py-10 text-11 font-bold tracking-t9 text-txt">
        {title}
      </div>
      {children}
    </section>
  );
}

export interface SettingsPageProps {
  onOpenPlans: () => void;
  onSignOut: () => void;
}

export function SettingsPage({ onOpenPlans, onSignOut }: SettingsPageProps) {
  const t = useT();
  const lang = useLang();
  const setLang = useI18nStore((s) => s.setLang);
  const prefs = usePreferencesStore();
  const session = useSessionStore();
  const flash = useBoardStore((s) => s.flash);
  const activeTasks = useBoardStore((s) => s.tasks.filter((task) => task.status === 'ACTIVE').length);

  return (
    <PageContainer width="settings" stacked={false}>
      <SettingsGroup title={t.interface}>
        <SettingsRow label={t.lang} note={t.langNote}>
          <Chip active={lang === 'EN'} size="md" onClick={() => setLang('EN')}>
            ENGLISH
          </Chip>
          <Chip active={lang === 'RU'} size="md" onClick={() => setLang('RU')}>
            РУССКИЙ
          </Chip>
        </SettingsRow>
        <SettingsRow label={t.density} note={t.densityNote}>
          <Chip
            active={prefs.density === 'compact'}
            size="md"
            onClick={() => prefs.setDensity('compact')}
          >
            {t.compact}
          </Chip>
          <Chip active={prefs.density === 'cozy'} size="md" onClick={() => prefs.setDensity('cozy')}>
            {t.cozy}
          </Chip>
        </SettingsRow>
        <SettingsRow label={t.subDefault} note={t.subDefaultNote}>
          <Chip
            active={prefs.subtasksExpandedByDefault}
            size="md"
            onClick={prefs.toggleSubtasksDefault}
          >
            {prefs.subtasksExpandedByDefault ? t.on : t.off}
          </Chip>
        </SettingsRow>
      </SettingsGroup>

      <SettingsGroup title={t.privacy}>
        <SettingsRow label="show_in_leaderboard" note={t.lbToggleNote}>
          <Chip
            active={session.user.showInLeaderboard}
            size="md"
            onClick={session.toggleLeaderboardVisibility}
          >
            {session.user.showInLeaderboard ? t.on : t.off}
          </Chip>
        </SettingsRow>
        <SettingsRow label={t.exportData} note={t.exportNote}>
          <Button
            variant="ghost"
            size="md"
            className="text-cyan"
            onClick={() => flash('EXPORT_QUEUED', 'quest_graph.json')}
          >
            {t.export}
          </Button>
        </SettingsRow>
      </SettingsGroup>

      <SettingsGroup title={t.integrations}>
        <SettingsRow label={t.gcal} note={t.gcalNote}>
          <Button
            variant="ghost"
            size="md"
            className="border-amber-border-soft text-amber"
            onClick={() => flash('FEATURE_NOT_SHIPPED', `${t.gcal} · ${t.soon}`)}
          >
            [ {t.soon} ]
          </Button>
        </SettingsRow>
      </SettingsGroup>

      <SettingsGroup title={t.account}>
        <SettingsRow label={t.username} note={t.accountNote} value={session.user.username} />
        <SettingsRow label={t.email} value={session.user.email} />
        <SettingsRow label={t.planRow} note={t.planRowNote}>
          <Button
            variant={session.user.plan === 'PRO' ? 'violet' : 'ghost'}
            size="md"
            className={session.user.plan === 'PRO' ? '' : 'text-amber'}
            onClick={onOpenPlans}
          >
            {session.user.plan === 'PRO'
              ? t.planProName
              : `${t.planFreeName} · ${activeTasks}/${FREE_TASK_CAP}`}
          </Button>
        </SettingsRow>
        <SettingsRow label={t.session} note={t.sessionNote}>
          <Button variant="ghost" size="md" className="text-cyan" onClick={onSignOut}>
            {t.signOut}
          </Button>
        </SettingsRow>
        <SettingsRow label={t.dangerZone} note={t.dangerNote}>
          <Button
            variant="danger"
            size="md"
            className="border-red-border text-red"
            onClick={() =>
              flash(
                'CONFIRMATION_REQUIRED',
                lang === 'RU'
                  ? 'действие требует подтверждения по email'
                  : 'action requires email confirmation',
              )
            }
          >
            {t.wipe}
          </Button>
        </SettingsRow>
      </SettingsGroup>
    </PageContainer>
  );
}
