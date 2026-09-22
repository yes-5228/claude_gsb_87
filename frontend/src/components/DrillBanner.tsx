// 明细列表顶部的筛选条件说明条。
//
// 从看板下钻进入时，用人类可读的方式说明当前指标口径与已带入的筛选条件，
// 并提供「返回看板」入口（带回看板原来的时间范围与片区）。
// 非下钻进入（chips 为空且没有来源标记）时不展示。
import { Link } from 'react-router-dom';

export interface FilterChip {
  label: string;
  value: string;
}

interface DrillBannerProps {
  /** 看板指标名称，例如「待验收任务」。 */
  metric?: string;
  chips: FilterChip[];
  /** 返回看板时保留的看板查询串。 */
  backHref?: string;
}

export function DrillBanner({ metric, chips, backHref }: DrillBannerProps) {
  const fromDashboard = Boolean(backHref);
  if (chips.length === 0 && !fromDashboard) {
    return null;
  }
  return (
    <div className="drill-banner">
      <div className="drill-banner-main">
        <span className="drill-banner-icon">📊</span>
        <div className="drill-banner-text">
          <p className="drill-banner-title">
            {fromDashboard ? (
              <>
                当前为看板指标
                {metric ? <strong>「{metric}」</strong> : null}
                的明细列表，条数与看板数字同口径
              </>
            ) : (
              <>当前筛选条件</>
            )}
          </p>
          {chips.length > 0 ? (
            <div className="drill-chips">
              {chips.map((chip) => (
                <span key={chip.label} className="drill-chip">
                  <span className="drill-chip-label">{chip.label}</span>
                  <span className="drill-chip-value">{chip.value}</span>
                </span>
              ))}
            </div>
          ) : (
            <p className="drill-banner-sub">未附加筛选条件，展示全部数据</p>
          )}
        </div>
      </div>
      {fromDashboard ? (
        <Link className="btn btn-ghost btn-sm" to={backHref ?? '/'}>
          ← 返回看板
        </Link>
      ) : null}
    </div>
  );
}
