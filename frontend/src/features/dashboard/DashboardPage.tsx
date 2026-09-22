// 运行看板：跨模块汇总管段、任务、清淤与验收数据。
//
// 全局筛选（片区 + 时间范围）保存在 URL 查询串中：
//   - 所有指标卡 / 状态分布 / 片区统计都可点击下钻到对应明细列表；
//   - 下钻自动带入状态、片区与时间条件，明细页总条数与看板数字同口径；
//   - 从明细页「返回看板」时通过 back 参数还原这里的时间范围。
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { dashboardApi, type DashboardFilter } from '../../api/dashboard';
import { segmentApi } from '../../api/pipesegments';
import { DataTable, type Column } from '../../components/DataTable';
import { PageHeader } from '../../components/PageHeader';
import { SectionCard } from '../../components/SectionCard';
import { StatCard } from '../../components/StatCard';
import { StateBlock } from '../../components/StateBlock';
import { useAsync } from '../../hooks/useAsync';
import { useMeta } from '../../providers/MetaProvider';
import type { DistrictStat, PendingAcceptanceItem, RecentRecordItem } from '../../types/domain';
import { formatDate, formatLength, formatNumber, formatPercent, formatVolume } from '../../utils/format';
import { optionLabel } from '../../utils/options';
import {
  drillAcceptances,
  drillHref,
  drillRecords,
  drillSegments,
  drillTasks,
  drillThisMonthRecords
} from './drill';

interface BarItem {
  label: string;
  value: number;
  href?: string;
}

function BarList({ items, emptyText }: { items: BarItem[]; emptyText: string }) {
  if (items.length === 0) {
    return <p className="form-note">{emptyText}</p>;
  }
  const max = Math.max(1, ...items.map((item) => item.value));
  return (
    <div className="bar-list">
      {items.map((item) => {
        const content = (
          <>
            <span>{item.label}</span>
            <div className="bar-track">
              <div className="bar-fill" style={{ width: `${(item.value / max) * 100}%` }} />
            </div>
            <span className="bar-value">{formatNumber(item.value, 0)}</span>
          </>
        );
        return item.href ? (
          <Link key={item.label} className="bar-row bar-row-link" to={item.href}>
            {content}
          </Link>
        ) : (
          <div key={item.label} className="bar-row">
            {content}
          </div>
        );
      })}
    </div>
  );
}

const pendingColumns: Column<PendingAcceptanceItem>[] = [
  { key: 'code', title: '任务编号', width: '140px', render: (row) => <span className="cell-main">{row.code}</span> },
  {
    key: 'title',
    title: '任务与管段',
    render: (row) => (
      <>
        <span>{row.title}</span>
        <span className="cell-sub">
          {row.segmentCode} · {row.segmentName}
        </span>
      </>
    )
  },
  { key: 'teamName', title: '实施班组', width: '120px', render: (row) => row.teamName || '—' },
  { key: 'finishedAt', title: '完工时间', width: '110px', render: (row) => formatDate(row.finishedAt) },
  {
    key: 'overdueDays',
    title: '超期',
    width: '90px',
    align: 'right',
    render: (row) =>
      row.overdueDays > 0 ? <span className="tag tag-danger">{row.overdueDays} 天</span> : <span className="tag tag-muted">正常</span>
  },
  {
    key: 'action',
    title: '操作',
    width: '90px',
    render: (row) => (
      <Link className="link" to={`/tasks/${row.taskId}`}>
        查看任务
      </Link>
    )
  }
];

const recentColumns: Column<RecentRecordItem>[] = [
  {
    key: 'code',
    title: '记录编号',
    width: '140px',
    render: (row) => (
      <Link className="cell-main" to={`/records/${row.recordId}`}>
        {row.code}
      </Link>
    )
  },
  { key: 'cleanedAt', title: '清淤日期', width: '110px', render: (row) => formatDate(row.cleanedAt) },
  {
    key: 'task',
    title: '所属任务与管段',
    render: (row) => (
      <>
        <span>{row.taskTitle}</span>
        <span className="cell-sub">
          {row.segmentCode} · {row.segmentName}
        </span>
      </>
    )
  },
  { key: 'lengthM', title: '清淤长度', width: '110px', align: 'right', render: (row) => formatLength(row.lengthM) },
  { key: 'sludgeVolumeM3', title: '清淤量', width: '110px', align: 'right', render: (row) => formatVolume(row.sludgeVolumeM3) }
];

export function DashboardPage() {
  const navigate = useNavigate();
  const { enums } = useMeta();
  const [searchParams, setSearchParams] = useSearchParams();

  const district = searchParams.get('district') ?? '';
  const dateFrom = searchParams.get('dateFrom') ?? '';
  const dateTo = searchParams.get('dateTo') ?? '';
  const filter: DashboardFilter = {
    district: district || undefined,
    dateFrom: dateFrom || undefined,
    dateTo: dateTo || undefined
  };
  const dashboardSearch = searchParams.toString();

  const overview = useAsync(() => dashboardApi.overview(filter), [district, dateFrom, dateTo]);
  const districts = useAsync(() => dashboardApi.districtStats(filter), [district, dateFrom, dateTo]);
  const pending = useAsync(() => dashboardApi.pendingAcceptance(6, filter), [district, dateFrom, dateTo]);
  const recent = useAsync(() => dashboardApi.recentRecords(6, filter), [district, dateFrom, dateTo]);
  const segmentOptions = useAsync(() => segmentApi.options(), []);

  const applyFilter = (patch: Record<string, string>) => {
    const next = new URLSearchParams(searchParams);
    Object.entries(patch).forEach(([key, value]) => {
      if (value) {
        next.set(key, value);
      } else {
        next.delete(key);
      }
    });
    setSearchParams(next);
  };

  const drill = (builder: () => { path: string; params: Record<string, string> }) => {
    const target = builder();
    navigate(drillHref(target));
  };

  const data = overview.data;
  const taskStatusBars: BarItem[] = enums
    ? enums.taskStatuses.map((status) => ({
        label: status.label,
        value: data?.taskByStatus?.[status.value] ?? 0,
        href: drillHref(
          drillTasks(filter, { metric: `${status.label}任务`, dashboardSearch }, { status: status.value })
        )
      }))
    : [];
  const segmentStatusBars: BarItem[] = enums
    ? enums.segmentStatuses.map((status) => ({
        label: status.label,
        value: data?.segmentByStatus?.[status.value] ?? 0,
        href: drillHref(
          drillSegments(filter, { metric: `${status.label}管段`, dashboardSearch }, { status: status.value })
        )
      }))
    : [];

  const districtColumns: Column<DistrictStat>[] = [
    {
      key: 'district',
      title: '片区',
      render: (row) => (
        <Link
          className="cell-main"
          to={drillHref(
            drillSegments(filter, { metric: `${row.district}片区管段`, dashboardSearch }, { district: row.district })
          )}
        >
          {row.district}
        </Link>
      )
    },
    {
      key: 'segmentCount',
      title: '管段',
      align: 'right',
      render: (row) => (
        <Link
          className="link"
          to={drillHref(
            drillSegments(filter, { metric: `${row.district}片区管段`, dashboardSearch }, { district: row.district })
          )}
        >
          {formatNumber(row.segmentCount, 0)}
        </Link>
      )
    },
    { key: 'segmentLengthM', title: '总长', align: 'right', render: (row) => formatLength(row.segmentLengthM) },
    {
      key: 'taskCount',
      title: '任务',
      align: 'right',
      render: (row) => (
        <Link
          className="link"
          to={drillHref(
            drillTasks(filter, { metric: `${row.district}片区任务`, dashboardSearch }, { district: row.district })
          )}
        >
          {formatNumber(row.taskCount, 0)}
        </Link>
      )
    },
    {
      key: 'sludgeVolumeM3',
      title: '清淤量',
      align: 'right',
      render: (row) => (
        <Link
          className="link"
          to={drillHref(
            drillRecords(filter, { metric: `${row.district}片区清淤记录`, dashboardSearch }, { district: row.district })
          )}
        >
          {formatVolume(row.sludgeVolumeM3)}
        </Link>
      )
    },
    { key: 'lastCleanedAt', title: '最近清淤', align: 'right', render: (row) => formatDate(row.lastCleanedAt) }
  ];

  const filterActive = Boolean(district || dateFrom || dateTo);

  return (
    <div className="page">
      <PageHeader
        title="运行看板"
        description="汇总管网台账、清淤任务、清淤记录与验收结论，用于掌握整体进度与待办事项。"
        actions={
          <>
            <button type="button" className="btn btn-ghost" onClick={() => navigate('/segments/new')}>
              新增管段
            </button>
            <button type="button" className="btn btn-primary" onClick={() => navigate('/tasks/new')}>
              登记清淤任务
            </button>
          </>
        }
      />

      <SectionCard title="筛选条件" subtitle="片区与时间范围会作用于全部可下钻指标，并随下钻自动带入明细页">
        <div className="filter-bar">
          <div className="filter-item">
            <span className="filter-label">所属片区</span>
            <select className="select" value={district} onChange={(event) => applyFilter({ district: event.target.value })}>
              <option value="">全部片区</option>
              {(segmentOptions.data?.districts ?? []).map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </div>
          <div className="filter-item">
            <span className="filter-label">时间范围起</span>
            <input
              className="input"
              type="date"
              value={dateFrom}
              onChange={(event) => applyFilter({ dateFrom: event.target.value })}
            />
          </div>
          <div className="filter-item">
            <span className="filter-label">时间范围止</span>
            <input
              className="input"
              type="date"
              value={dateTo}
              onChange={(event) => applyFilter({ dateTo: event.target.value })}
            />
          </div>
          <div className="filter-actions">
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() => setSearchParams(new URLSearchParams())}
              disabled={!filterActive}
            >
              重置
            </button>
          </div>
        </div>
        <p className="form-note filter-scope-note">
          时间范围口径：清淤任务按计划开始日期、清淤记录按清淤日期、验收记录按验收日期统计；管段台账为当前快照，只受片区筛选影响。
        </p>
      </SectionCard>

      <StateBlock loading={overview.loading} error={overview.error} onRetry={overview.reload}>
        <div className="stat-grid">
          <StatCard
            label="管段总数"
            value={formatNumber(data?.segmentTotal ?? 0, 0)}
            hint={`总长度 ${formatLength(data?.segmentTotalLengthM ?? 0)}`}
            tone="primary"
            onClick={() =>
              drill(() => drillSegments(filter, { metric: '管段总数', dashboardSearch }))
            }
          />
          <StatCard
            label="未清淤管段"
            value={formatNumber(data?.uncleanedSegmentCount ?? 0, 0)}
            hint="尚无验收合格清淤记录的管段（台账快照口径）"
            tone={data && data.uncleanedSegmentCount > 0 ? 'warn' : 'success'}
            onClick={() =>
              drill(() =>
                drillSegments(filter, { metric: '未清淤管段', dashboardSearch }, { uncleaned: 'true' })
              )
            }
          />
          <StatCard
            label="清淤任务"
            value={formatNumber(data?.taskTotal ?? 0, 0)}
            hint={`待开工 ${data?.taskByStatus?.pending ?? 0} · 清淤中 ${data?.taskByStatus?.in_progress ?? 0}`}
            onClick={() => drill(() => drillTasks(filter, { metric: '清淤任务', dashboardSearch }))}
          />
          <StatCard
            label="超期任务"
            value={formatNumber(data?.taskOverdue ?? 0, 0)}
            hint="计划完成日期已过，仍为待开工 / 清淤中"
            tone={data && data.taskOverdue > 0 ? 'danger' : 'default'}
            onClick={() =>
              drill(() => drillTasks(filter, { metric: '超期任务', dashboardSearch }, { overdue: 'true' }))
            }
          />
          <StatCard
            label="待验收任务"
            value={formatNumber(data?.pendingAcceptanceCount ?? 0, 0)}
            hint="已完工报验，等待验收结论"
            tone={data && data.pendingAcceptanceCount > 0 ? 'warn' : 'default'}
            onClick={() =>
              drill(() =>
                drillTasks(filter, { metric: '待验收任务', dashboardSearch }, { status: 'completed' })
              )
            }
          />
          <StatCard
            label="清淤记录"
            value={formatNumber(data?.recordTotal ?? 0, 0)}
            hint={`累计清淤长度 ${formatLength(data?.cleanedLengthM ?? 0)}`}
            onClick={() => drill(() => drillRecords(filter, { metric: '清淤记录', dashboardSearch }))}
          />
          <StatCard
            label="累计清淤量"
            value={formatVolume(data?.sludgeTotalM3 ?? 0)}
            hint="按筛选范围内清淤记录汇总"
            onClick={() => drill(() => drillRecords(filter, { metric: '累计清淤量明细', dashboardSearch }))}
          />
          <StatCard
            label="本月清淤量"
            value={formatVolume(data?.sludgeThisMonthM3 ?? 0)}
            hint="自然月清淤日期口径，下钻固定本月区间"
            onClick={() =>
              drill(() => drillThisMonthRecords(filter, { metric: '本月清淤量', dashboardSearch }))
            }
          />
          <StatCard
            label="验收总次数"
            value={formatNumber(data?.acceptanceTotal ?? 0, 0)}
            hint="筛选范围内的验收记录"
            onClick={() => drill(() => drillAcceptances(filter, { metric: '验收记录', dashboardSearch }))}
          />
          <StatCard
            label="验收合格率"
            value={formatPercent(data?.acceptancePassRate ?? 0)}
            hint={`合格 ${data?.acceptancePassCount ?? 0} / 共 ${data?.acceptanceTotal ?? 0} 次`}
            tone="success"
            onClick={() =>
              drill(() =>
                drillAcceptances(filter, { metric: '合格验收', dashboardSearch }, { result: 'pass' })
              )
            }
          />
          <StatCard
            label="待整改验收"
            value={formatNumber(data?.pendingRectifyCount ?? 0, 0)}
            hint="结论为需整改且尚未登记整改完成"
            tone={data && data.pendingRectifyCount > 0 ? 'warn' : 'default'}
            onClick={() =>
              drill(() =>
                drillAcceptances(
                  filter,
                  { metric: '待整改验收', dashboardSearch },
                  { pendingRectify: 'true' }
                )
              )
            }
          />
        </div>
      </StateBlock>

      <div className="panel-grid panel-grid-wide">
        <SectionCard
          title="待验收任务"
          subtitle="已完工报验但尚无验收结论的任务"
          extra={
            <Link
              className="link"
              to={drillHref(
                drillTasks(filter, { metric: '待验收任务', dashboardSearch }, { status: 'completed' })
              )}
            >
              全部待验收
            </Link>
          }
        >
          <div className="card-body-flush">
            <DataTable
              columns={pendingColumns}
              rows={pending.data ?? []}
              rowKey={(row) => row.taskId}
              loading={pending.loading}
              error={pending.error}
              onRetry={pending.reload}
              emptyText="暂无待验收任务"
            />
          </div>
        </SectionCard>

        <SectionCard title="任务状态分布" subtitle="按清淤任务状态统计，点击数字可下钻">
          <BarList items={taskStatusBars} emptyText="暂无任务数据" />
          <div style={{ height: 16 }} />
          <p className="form-note">管段运行状态（台账快照，仅受片区筛选影响）</p>
          <div style={{ height: 8 }} />
          <BarList items={segmentStatusBars} emptyText="暂无管段数据" />
        </SectionCard>
      </div>

      <div className="panel-grid panel-grid-wide">
        <SectionCard
          title="最近清淤记录"
          subtitle="按清淤日期倒序展示最新录入结果"
          extra={
            <Link className="link" to={drillHref(drillRecords(filter, { metric: '清淤记录', dashboardSearch }))}>
              查看全部记录
            </Link>
          }
        >
          <div className="card-body-flush">
            <DataTable
              columns={recentColumns}
              rows={recent.data ?? []}
              rowKey={(row) => row.recordId}
              loading={recent.loading}
              error={recent.error}
              onRetry={recent.reload}
              emptyText="暂无清淤记录"
            />
          </div>
        </SectionCard>

        <SectionCard title="分片区统计" subtitle="管段列为台账快照，任务与清淤量随时间范围变化">
          <div className="card-body-flush">
            <DataTable
              columns={districtColumns}
              rows={districts.data ?? []}
              rowKey={(row) => row.district}
              loading={districts.loading}
              error={districts.error}
              onRetry={districts.reload}
              emptyText="暂无片区数据"
            />
          </div>
        </SectionCard>
      </div>

      <p className="form-note">
        说明：验收合格率 = 合格验收次数 / 验收总次数；未清淤管段指尚无「验收合格」记录的管段，
        与管段台账中的最近清淤日期口径一致。所有指标卡与状态分布均可点击下钻，明细页条数与看板数字使用同一套后端筛选口径；
        字典标签取自后端 {optionLabel(enums?.acceptanceResults, 'pass')} 等统一枚举。
      </p>
    </div>
  );
}
