// 明细列表读取下钻参数的通用辅助。
//
// 从看板下钻时 URL 上带 from=dashboard + metric=<指标名>，
// 返回看板时用 back 参数（编码后的看板查询串）保留原时间范围与片区。
import { useSearchParams } from 'react-router-dom';

export interface DrillContext {
  fromDashboard: boolean;
  metric: string;
  backHref: string;
}

export function useDrillContext(): DrillContext {
  const [params] = useSearchParams();
  const fromDashboard = params.get('from') === 'dashboard';
  const metric = params.get('metric') ?? '';
  const back = params.get('back');
  return {
    fromDashboard,
    metric,
    backHref: back ? `/?${back}` : '/'
  };
}

/**
 * 把看板当前查询串编码为下钻链接的 back 参数。
 *
 * 传入的可能是已编码串（searchParams.toString() 的结果），先解码再编码，
 * 避免 URLSearchParams 二次拼装时出现双重编码。
 */
export function encodeBack(dashboardSearch: string): string {
  const raw = dashboardSearch.startsWith('?') ? dashboardSearch.slice(1) : dashboardSearch;
  try {
    return encodeURIComponent(decodeURIComponent(raw));
  } catch {
    return encodeURIComponent(raw);
  }
}

/**
 * 手动修改筛选条件时移除下钻标记。
 *
 * 一旦用户在明细页调整了任一筛选条件，结果就不再严格等于看板指标，
 * 此时应去掉「看板下钻」说明与返回链接，避免口径误导；翻页不移除。
 */
export function stripDrillParams(params: URLSearchParams): URLSearchParams {
  params.delete('from');
  params.delete('metric');
  params.delete('back');
  return params;
}
