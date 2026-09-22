import type { DistrictStat, Overview, PendingAcceptanceItem, RecentRecordItem } from '../types/domain';
import { buildQuery, http } from './client';

export interface DashboardFilter {
  district?: string;
  dateFrom?: string;
  dateTo?: string;
}

export const dashboardApi = {
  overview: (filter: DashboardFilter = {}) =>
    http.get<Overview>(`/dashboard/overview${buildQuery({ ...filter })}`),
  districtStats: (filter: DashboardFilter = {}) =>
    http.get<DistrictStat[]>(`/dashboard/district-stats${buildQuery({ ...filter })}`),
  pendingAcceptance: (limit = 8, filter: DashboardFilter = {}) =>
    http.get<PendingAcceptanceItem[]>(
      `/dashboard/pending-acceptance${buildQuery({ limit, ...filter })}`
    ),
  recentRecords: (limit = 8, filter: DashboardFilter = {}) =>
    http.get<RecentRecordItem[]>(`/dashboard/recent-records${buildQuery({ limit, ...filter })}`)
};
