package dashboard_test

import (
	"context"
	"testing"

	"github.com/drainage/desilting/internal/modules/acceptance"
	"github.com/drainage/desilting/internal/modules/cleaningrecord"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/dashboard"
	"github.com/drainage/desilting/internal/modules/pipesegment"
	"github.com/drainage/desilting/internal/shared/date"
	"github.com/drainage/desilting/internal/testsupport"
)

// setup 按生产装配方式构造看板服务（依赖图与 router.Setup 一致）。
func setup(t *testing.T) (*testsupport.Services, *dashboard.Service) {
	t.Helper()
	db := testsupport.NewDB(t)
	services := testsupport.NewServices(db)
	svc := dashboard.NewService(
		db,
		services.Segments,
		services.Tasks,
		services.Records,
		services.Acceptances,
	)
	return services, svc
}

// accept 走完整流程让任务验收合格：录入记录 -> 完工报验 -> 验收合格。
func accept(t *testing.T, services *testsupport.Services, segmentID uint, title string, sludge float64) {
	t.Helper()
	ctx := context.Background()
	task := services.CreateTask(t, segmentID, title)
	services.CreateRecord(t, task.ID, sludge)
	if _, err := services.Tasks.Complete(ctx, task.ID); err != nil {
		t.Fatalf("完工报验失败: %v", err)
	}
	if _, err := services.Acceptances.Create(ctx, testsupport.PassRequest(task.ID, 90)); err != nil {
		t.Fatalf("验收合格登记失败: %v", err)
	}
}

// complete 录入记录并完工报验，任务停留在「待验收」。
func complete(t *testing.T, services *testsupport.Services, segmentID uint, title string, sludge float64) {
	t.Helper()
	ctx := context.Background()
	task := services.CreateTask(t, segmentID, title)
	services.CreateRecord(t, task.ID, sludge)
	if _, err := services.Tasks.Complete(ctx, task.ID); err != nil {
		t.Fatalf("完工报验失败: %v", err)
	}
}

// rework 录入记录、完工报验后登记「需整改」且不登记整改完成。
func rework(t *testing.T, services *testsupport.Services, segmentID uint, title string, sludge float64) {
	t.Helper()
	ctx := context.Background()
	task := services.CreateTask(t, segmentID, title)
	services.CreateRecord(t, task.ID, sludge)
	if _, err := services.Tasks.Complete(ctx, task.ID); err != nil {
		t.Fatalf("完工报验失败: %v", err)
	}
	if _, err := services.Acceptances.Create(ctx, testsupport.ReworkRequest(task.ID)); err != nil {
		t.Fatalf("需整改验收登记失败: %v", err)
	}
}

// TestOverviewMatchesListTotals 核心一致性：每个可下钻指标都必须与对应明细列表的总条数相等。
func TestOverviewMatchesListTotals(t *testing.T) {
	services, svc := setup(t)
	ctx := context.Background()

	// 三个管段，分布在两个片区；east1 下挂多条任务，验证管段基数不被任务行数带偏。
	east1 := services.CreateSegment(t, "PS-EAST-001", "城东片区")
	east2 := services.CreateSegment(t, "PS-EAST-002", "城东片区")
	west1 := services.CreateSegment(t, "PS-WEST-001", "城西片区")
	fresh := services.CreateSegment(t, "PS-EAST-003", "城东片区") // 从不清淤

	accept(t, services, east1.ID, "城东1号已验收任务", 10)
	complete(t, services, east1.ID, "城东1号待验收任务", 5)
	rework(t, services, east2.ID, "城东2号需整改任务", 7)
	accept(t, services, west1.ID, "城西已验收任务", 3)

	overview, err := svc.Overview(ctx, dashboard.Filter{})
	testsupport.RequireNoError(t, err)

	// 无筛选：指标数字必须与各模块列表 total 完全一致。
	if _, total, err := services.Segments.List(ctx, pipesegment.ListQuery{}); err != nil {
		t.Fatalf("查询管段列表失败: %v", err)
	} else if overview.SegmentTotal != total || total != 4 {
		t.Fatalf("管段总数看板=%d 列表=%d，期望均为 4", overview.SegmentTotal, total)
	}

	if _, total, err := services.Tasks.List(ctx, cleaningtask.ListQuery{}); err != nil {
		t.Fatalf("查询任务列表失败: %v", err)
	} else if overview.TaskTotal != total || total != 4 {
		t.Fatalf("任务总数看板=%d 列表=%d，期望均为 4", overview.TaskTotal, total)
	}

	if _, total, err := services.Records.List(ctx, cleaningrecord.ListQuery{}); err != nil {
		t.Fatalf("查询记录列表失败: %v", err)
	} else if overview.RecordTotal != total || total != 4 {
		t.Fatalf("清淤记录数看板=%d 列表=%d，期望均为 4", overview.RecordTotal, total)
	}

	if _, total, err := services.Acceptances.List(ctx, acceptance.ListQuery{}); err != nil {
		t.Fatalf("查询验收列表失败: %v", err)
	} else if overview.AcceptanceTotal != total || total != 3 {
		t.Fatalf("验收总数看板=%d 列表=%d，期望均为 3", overview.AcceptanceTotal, total)
	}

	// 待验收任务 = status=completed 的任务条数。
	if _, total, err := services.Tasks.List(ctx, cleaningtask.ListQuery{Status: cleaningtask.StatusCompleted}); err != nil {
		t.Fatalf("查询待验收任务失败: %v", err)
	} else if overview.PendingAcceptanceCount != total || total != 1 {
		t.Fatalf("待验收任务看板=%d 列表=%d，期望均为 1", overview.PendingAcceptanceCount, total)
	}

	// 合格验收 = result=pass 的验收条数。
	if _, total, err := services.Acceptances.List(ctx, acceptance.ListQuery{Result: acceptance.ResultPass}); err != nil {
		t.Fatalf("查询合格验收失败: %v", err)
	} else if overview.AcceptancePassCount != total || total != 2 {
		t.Fatalf("合格验收看板=%d 列表=%d，期望均为 2", overview.AcceptancePassCount, total)
	}

	// 待整改 = pendingRectify=true 的验收条数。
	if _, total, err := services.Acceptances.List(ctx, acceptance.ListQuery{PendingRectify: true}); err != nil {
		t.Fatalf("查询待整改验收失败: %v", err)
	} else if overview.PendingRectifyCount != total || total != 1 {
		t.Fatalf("待整改验收看板=%d 列表=%d，期望均为 1", overview.PendingRectifyCount, total)
	}

	// 未清淤管段 = last_cleaned_at IS NULL：
	// east1 / west1 已验收合格而清淤；east2 仅「需整改」未合格、fresh 从不清淤，二者都算未清淤。
	if _, total, err := services.Segments.List(ctx, pipesegment.ListQuery{Uncleaned: true}); err != nil {
		t.Fatalf("查询未清淤管段失败: %v", err)
	} else if overview.UncleanedSegmentCount != total || total != 2 {
		t.Fatalf("未清淤管段看板=%d 列表=%d，期望均为 2（%s 与需整改未合格的管段）",
			overview.UncleanedSegmentCount, total, fresh.Code)
	}
}

// TestOverviewDistrictFilter 片区筛选：指标随片区收窄，且与带 district 的列表条数一致。
func TestOverviewDistrictFilter(t *testing.T) {
	services, svc := setup(t)
	ctx := context.Background()

	east := services.CreateSegment(t, "PS-E-001", "城东片区")
	west := services.CreateSegment(t, "PS-W-001", "城西片区")

	eastTask := services.CreateTask(t, east.ID, "城东任务")
	services.CreateRecord(t, eastTask.ID, 10)
	westTask := services.CreateTask(t, west.ID, "城西任务")
	services.CreateRecord(t, westTask.ID, 20)

	overview, err := svc.Overview(ctx, dashboard.Filter{District: "城东片区"})
	testsupport.RequireNoError(t, err)

	if overview.SegmentTotal != 1 {
		t.Fatalf("城东片区管段应为 1，实际 %d", overview.SegmentTotal)
	}
	if _, total, err := services.Tasks.List(ctx, cleaningtask.ListQuery{District: "城东片区"}); err != nil {
		t.Fatalf("查询城东任务失败: %v", err)
	} else if overview.TaskTotal != total || total != 1 {
		t.Fatalf("城东任务看板=%d 列表=%d，期望均为 1", overview.TaskTotal, total)
	}
	if _, total, err := services.Records.List(ctx, cleaningrecord.ListQuery{District: "城东片区"}); err != nil {
		t.Fatalf("查询城东记录失败: %v", err)
	} else if overview.RecordTotal != total || total != 1 {
		t.Fatalf("城东记录看板=%d 列表=%d，期望均为 1", overview.RecordTotal, total)
	}
}

// TestOverviewTimeFilter 时间范围：任务按计划开始日期、记录按清淤日期、验收按验收日期过滤。
func TestOverviewTimeFilter(t *testing.T) {
	services, svc := setup(t)
	ctx := context.Background()
	seg := services.CreateSegment(t, "PS-T-001", "城东片区")

	// testsupport 固定任务计划开始日期为今天-3天、记录清淤日期为昨天。
	oldTask := services.CreateTask(t, seg.ID, "较早的任务")
	services.CreateRecord(t, oldTask.ID, 10)

	// 未来区间：任务和记录都不应计入。
	futureFrom := date.Today().AddDays(10)
	futureTo := date.Today().AddDays(20)
	overview, err := svc.Overview(ctx, dashboard.Filter{DateFrom: &futureFrom, DateTo: &futureTo})
	testsupport.RequireNoError(t, err)
	if overview.TaskTotal != 0 {
		t.Fatalf("未来时间范围内任务应为 0，实际 %d", overview.TaskTotal)
	}
	if overview.RecordTotal != 0 {
		t.Fatalf("未来时间范围内记录应为 0，实际 %d", overview.RecordTotal)
	}

	// 覆盖全部历史数据的区间：看板与带相同时间条件的列表条数一致。
	pastFrom := date.Today().AddDays(-365)
	pastTo := date.Today()
	overview, err = svc.Overview(ctx, dashboard.Filter{DateFrom: &pastFrom, DateTo: &pastTo})
	testsupport.RequireNoError(t, err)
	if _, total, err := services.Tasks.List(ctx, cleaningtask.ListQuery{PlanFrom: &pastFrom, PlanTo: &pastTo}); err != nil {
		t.Fatalf("查询区间任务失败: %v", err)
	} else if overview.TaskTotal != total || total != 1 {
		t.Fatalf("区间任务看板=%d 列表=%d，期望均为 1", overview.TaskTotal, total)
	}
	if _, total, err := services.Records.List(ctx, cleaningrecord.ListQuery{DateFrom: &pastFrom, DateTo: &pastTo}); err != nil {
		t.Fatalf("查询区间记录失败: %v", err)
	} else if overview.RecordTotal != total || total != 1 {
		t.Fatalf("区间记录看板=%d 列表=%d，期望均为 1", overview.RecordTotal, total)
	}

	// 管段台账为快照口径，不受时间范围影响。
	if overview.SegmentTotal != 1 {
		t.Fatalf("管段台账不应受时间范围影响，期望 1，实际 %d", overview.SegmentTotal)
	}
}

// TestDistrictStatsDedupesSegmentBase 同一管段多条任务 / 多条记录时，
// 片区统计中的管段基数只能计一次，不能被任务或记录行数带偏。
func TestDistrictStatsDedupesSegmentBase(t *testing.T) {
	services, svc := setup(t)
	ctx := context.Background()
	seg := services.CreateSegment(t, "PS-D-001", "城东片区")

	// 同一管段下 3 条任务、每条任务 2 条记录，管段基数仍必须是 1。
	for range 3 {
		task := services.CreateTask(t, seg.ID, "重复基数任务")
		services.CreateRecord(t, task.ID, 4)
		services.CreateRecord(t, task.ID, 6)
	}

	stats, err := svc.DistrictStats(ctx, dashboard.Filter{})
	testsupport.RequireNoError(t, err)
	if len(stats) != 1 {
		t.Fatalf("期望 1 个片区统计，实际 %d", len(stats))
	}
	if stats[0].SegmentCount != 1 {
		t.Fatalf("同一管段多任务下管段基数应为 1，实际 %d（被明细行数带偏）", stats[0].SegmentCount)
	}
	if stats[0].TaskCount != 3 {
		t.Fatalf("片区任务数应为 3，实际 %d", stats[0].TaskCount)
	}
	// 6 条记录：每条任务 4+6=10，共 30。
	if stats[0].SludgeVolumeM3 != 30 {
		t.Fatalf("片区清淤量应为 30，实际 %v", stats[0].SludgeVolumeM3)
	}
}
