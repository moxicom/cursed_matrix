import { useEffect } from 'react';
import { RouterProvider } from 'react-router-dom';

import { useBoardStore } from '@/features/board/board.store';
import { useSessionStore } from '@/features/session/session.store';

import { router } from './router';

export function App() {
  const restore = useSessionStore((s) => s.restore);
  const status = useSessionStore((s) => s.status);
  const loadBoard = useBoardStore((s) => s.load);

  // The session lives in a cookie the page cannot read, so the only way to
  // know whether there is one is to ask — once, before anything routes on it.
  useEffect(() => {
    void restore();
  }, [restore]);

  // The board is fetched once there is somebody to fetch it for; before that
  // every request would be a 401.
  useEffect(() => {
    if (status === 'authenticated') void loadBoard();
  }, [status, loadBoard]);

  return <RouterProvider router={router} />;
}
