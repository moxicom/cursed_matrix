import { useId, useState } from 'react';

import { useSessionStore } from '@/features/session/session.store';
import { messageForCode } from '@/shared/api/messages';
import { useLang, useT } from '@/shared/i18n';
import { Button, FieldLabel, Input, Panel } from '@/shared/ui';

export interface SignInPageProps {
  onDone: () => void;
}

type Mode = 'signIn' | 'register';

/**
 * The only screen that takes a password.
 *
 * Nothing is kept here once it is sent: the session comes back as cookies the
 * page cannot read, so there is no token to hold and nothing for an injected
 * script to find.
 */
export function SignInPage({ onDone }: SignInPageProps) {
  const t = useT();
  const lang = useLang();
  const { signIn, register, busy, error } = useSessionStore();

  // A real <label for> needs an id, and the page can appear more than once in
  // a tree during a transition, so the id is generated rather than fixed.
  const fieldId = useId();
  const [mode, setMode] = useState<Mode>('signIn');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    if (busy) return;

    const attempt =
      mode === 'signIn'
        ? signIn(username.trim(), password)
        : register(username.trim(), password, lang);

    // Fired, not awaited: the form is not a place to block, and the store
    // already holds the busy flag and the refusal.
    void attempt.then((ok) => {
      if (ok) onDone();
    });
  };

  return (
    <div className="flex min-h-0 flex-1 items-center justify-center overflow-y-auto bg-bg-base px-26 py-40">
      <Panel className="w-full max-w-[420px] p-26">
        <h1 className="mb-6 text-130 text-txt-bright">
          {mode === 'signIn' ? t.signInTitle : t.registerTitle}
        </h1>
        <p className="mb-22 text-95 leading-[1.6] text-txt-faint">
          {mode === 'signIn' ? t.signInSub : t.registerSub}
        </p>

        <form onSubmit={submit} className="flex flex-col gap-16">
          <div>
            <FieldLabel htmlFor={`${fieldId}-username`}>{t.username}</FieldLabel>
            <Input
              id={`${fieldId}-username`}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              required
              minLength={3}
              maxLength={32}
            />
          </div>

          <div>
            <FieldLabel htmlFor={`${fieldId}-password`}>{t.password}</FieldLabel>
            <Input
              id={`${fieldId}-password`}
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              // Told to the browser so it offers the right entry, and so a
              // new account is offered a generated one.
              autoComplete={mode === 'signIn' ? 'current-password' : 'new-password'}
              required
              minLength={12}
              maxLength={128}
            />
            {mode === 'register' ? (
              <p className="mt-6 text-90 text-txt-faint">{t.passwordRule}</p>
            ) : null}
          </div>

          {error ? (
            <p role="alert" className="text-95 text-rose">
              {messageForCode(error.code, error.params, t)}
            </p>
          ) : null}

          <Button type="submit" variant="primary" size="lg" disabled={busy}>
            {busy ? t.working : mode === 'signIn' ? t.signInAction : t.registerAction}
          </Button>
        </form>

        <button
          type="button"
          className="mt-18 text-95 text-txt-faint underline-offset-4 hover:text-txt-bright hover:underline"
          onClick={() => setMode(mode === 'signIn' ? 'register' : 'signIn')}
        >
          {mode === 'signIn' ? t.registerSwitch : t.signInSwitch}
        </button>
      </Panel>
    </div>
  );
}
