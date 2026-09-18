import { createBrowserRouter } from 'react-router-dom';

import { AppLayout } from './AppLayout';
import { RequireAccess } from './RequireAccess';
import {
  ActivityRoute,
  BoardRoute,
  GraphRoute,
  LandingRoute,
  LeaderboardRoute,
  NotFoundRoute,
  PricingRoute,
  ProfileRoute,
  SettingsRoute,
} from './routes';
import { UiKitPage } from '@/pages/uikit/UiKitPage';
import { ROUTES } from '@/shared/config/navigation';

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      // public
      { path: ROUTES.landing, element: <LandingRoute /> },
      { path: ROUTES.upgrade, element: <PricingRoute /> },

      // everything below needs an active subscription
      {
        element: <RequireAccess />,
        children: [
          { path: ROUTES.board, element: <BoardRoute /> },
          { path: ROUTES.graph, element: <GraphRoute /> },
          { path: ROUTES.activity, element: <ActivityRoute /> },
          { path: ROUTES.leaderboard, element: <LeaderboardRoute /> },
          { path: ROUTES.profile, element: <ProfileRoute /> },
          { path: ROUTES.settings, element: <SettingsRoute /> },
        ],
      },

      // component gallery, kept out of the navigation
      { path: '/uikit', element: <UiKitPage /> },

      { path: '*', element: <NotFoundRoute /> },
    ],
  },
]);
