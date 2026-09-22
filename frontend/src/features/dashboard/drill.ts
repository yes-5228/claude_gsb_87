// 看板下钻：构造跳转到各明细列表的查询参数。
//
// 看板的全局筛选（片区 + 时间范围）会自动带入明细列表；
// 时间范围在各业务域使用不同的落库字段，因此映射为对应列表的查询参数：
//   任务列表：planFrom/planTo（计划开始日期）
//   清淤记录：dateFrom/dateTo（清淤日期）
//   验收列表：dateFrom/dateTo（验收日期）
//   管段列表：台账快照不按时间过滤，因此不带时间参数
//
// 额外统一携带 from=dashboard、metric=<指标名>、back=<看板原查询串>，
// 明细页据此说明当前筛选条件，并在返回看板时保留原时间范围。
import type { DashboardFilter } from '../../api/dashboard';
import { encodeBack } from '../../hooks/useDrillContext';

export interface DrillTarget {
  path: string;
  params: Record<string, string>;
}

interface DrillOptions {
  metric: string;
  /** 看板当前的查询串（形如 ?district=城东&dateFrom=...），用于返回时还原。 */
  dashboardSearch: string;
}

function withMeta(
  params: Record<string, string>,
  filter: DashboardFilter,
  options: DrillOptions
): Record<string, string> {
  const result: Record<string, string> = { ...params };
  if (filter.district) {
    result.district = filter.district;
  }
  result.from = 'dashboard';
  result.metric = options.metric;
  const back = encodeBack(options.dashboardSearch);
  if (back) {
    result.back = back;
  }
  return result;
}

function timeParams(
  filter: DashboardFilter,
  fromKey: 'planFrom' | 'dateFrom',
  toKey: 'planTo' | 'dateTo'
): Record<string, string> {
  const params: Record<string, string> = {};
  if (filter.dateFrom) {
    params[fromKey] = filter.dateFrom;
  }
  if (filter.dateTo) {
    params[toKey] = filter.dateTo;
  }
  return params;
}

/** 管段台账指标下钻。status / uncleaned 为指标自带的状态条件。 */
export function drillSegments(
  filter: DashboardFilter,
  options: DrillOptions,
  extra: Record<string, string> = {}
): DrillTarget {
  return { path: '/segments', params: withMeta(extra, filter, options) };
}

/** 清淤任务指标下钻。 */
export function drillTasks(
  filter: DashboardFilter,
  options: DrillOptions,
  extra: Record<string, string> = {}
): DrillTarget {
  return {
    path: '/tasks',
    params: withMeta(
      { ...timeParams(filter, 'planFrom', 'planTo'), ...extra },
      filter,
      options
    )
  };
}

/** 清淤记录指标下钻。 */
export function drillRecords(
  filter: DashboardFilter,
  options: DrillOptions,
  extra: Record<string, string> = {}
): DrillTarget {
  return {
    path: '/records',
    params: withMeta(
      { ...timeParams(filter, 'dateFrom', 'dateTo'), ...extra },
      filter,
      options
    )
  };
}

/** 验收记录指标下钻。 */
export function drillAcceptances(
  filter: DashboardFilter,
  options: DrillOptions,
  extra: Record<string, string> = {}
): DrillTarget {
  return {
    path: '/acceptances',
    params: withMeta(
      { ...timeParams(filter, 'dateFrom', 'dateTo'), ...extra },
      filter,
      options
    )
  };
}

/** 本月清淤量下钻：时间口径固定为自然月（忽略看板自定义时间范围），仅继承片区。 */
export function drillThisMonthRecords(filter: DashboardFilter, options: DrillOptions): DrillTarget {
  const now = new Date();
  const first = new Date(now.getFullYear(), now.getMonth(), 1);
  const last = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  const monthFilter: DashboardFilter = { district: filter.district };
  return drillRecords(
    monthFilter,
    options,
    {
      dateFrom: first.toISOString().slice(0, 10),
      dateTo: last.toISOString().slice(0, 10)
    }
  );
}

/** 把下钻目标拼成可用于 navigate / Link 的路径。 */
export function drillHref(target: DrillTarget): string {
  const search = new URLSearchParams(target.params).toString();
  return search ? `${target.path}?${search}` : target.path;
}
