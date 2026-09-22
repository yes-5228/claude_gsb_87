// Package dashboard 汇总看板模块：跨模块只读统计，不写入任何业务数据。
//
// 看板上的每个可下钻指标都直接复用各业务模块列表查询的同一套筛选谓词
// （通过模块 Service 的 Count/CountGrouped/Sum 方法），因此看板数字与下钻
// 明细列表的总条数始终同口径：指标口径一旦调整，明细结果自动同步变化。
package dashboard

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/acceptance"
	"github.com/drainage/desilting/internal/modules/cleaningrecord"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/pipesegment"
	"github.com/drainage/desilting/internal/shared/date"
	"github.com/drainage/desilting/internal/shared/num"
	"github.com/drainage/desilting/internal/shared/refx"
)

// 各业务模块对看板开放的统计能力（由生产环境的模块 Service 实现）。
// 只依赖统计方法，避免把整个模块的写操作接口耦合进看板。
type (
	segmentGateway interface {
		CountForDashboard(ctx context.Context, query pipesegment.ListQuery) (int64, error)
		SumLengthForDashboard(ctx context.Context, query pipesegment.ListQuery) (float64, error)
		CountGroupedForDashboard(ctx context.Context, query pipesegment.ListQuery, column string) (map[string]int64, error)
	}
	taskGateway interface {
		CountForDashboard(ctx context.Context, query cleaningtask.ListQuery) (int64, error)
		CountGroupedForDashboard(ctx context.Context, query cleaningtask.ListQuery, column string) (map[string]int64, error)
	}
	recordGateway interface {
		CountForDashboard(ctx context.Context, query cleaningrecord.ListQuery) (int64, error)
		SumForDashboard(ctx context.Context, query cleaningrecord.ListQuery) (cleaningrecord.RecordSum, error)
	}
	acceptanceGateway interface {
		CountForDashboard(ctx context.Context, query acceptance.ListQuery) (int64, error)
		CountGroupedForDashboard(ctx context.Context, query acceptance.ListQuery, column string) (map[string]int64, error)
	}
)

// Filter 看板的全局筛选条件，来自请求 query。
//
// 时间范围在各业务域的落库口径：
//   - 清淤任务：计划开始日期（与任务列表的 planFrom/planTo 一致）
//   - 清淤记录：清淤日期（与记录列表的 dateFrom/dateTo 一致）
//   - 验收记录：验收日期（与验收列表的 dateFrom/dateTo 一致）
//   - 管段台账：快照口径，不随时间范围变化（管段没有业务发生时间）
type Filter struct {
	District string
	DateFrom *date.Date
	DateTo   *date.Date
}

// Service 看板统计。
type Service struct {
	db          *gorm.DB
	segments    segmentGateway
	tasks       taskGateway
	records     recordGateway
	acceptances acceptanceGateway
}

// NewService 构造服务。
func NewService(
	db *gorm.DB,
	segments segmentGateway,
	tasks taskGateway,
	records recordGateway,
	acceptances acceptanceGateway,
) *Service {
	return &Service{db: db, segments: segments, tasks: tasks, records: records, acceptances: acceptances}
}

// Overview 总览指标。
type Overview struct {
	SegmentTotal          int64            `json:"segmentTotal"`
	SegmentTotalLengthM   float64          `json:"segmentTotalLengthM"`
	SegmentByStatus       map[string]int64 `json:"segmentByStatus"`
	UncleanedSegmentCount int64            `json:"uncleanedSegmentCount"`

	TaskTotal    int64            `json:"taskTotal"`
	TaskByStatus map[string]int64 `json:"taskByStatus"`
	TaskOverdue  int64            `json:"taskOverdue"`

	RecordTotal       int64   `json:"recordTotal"`
	SludgeTotalM3     float64 `json:"sludgeTotalM3"`
	SludgeThisMonthM3 float64 `json:"sludgeThisMonthM3"`
	CleanedLengthM    float64 `json:"cleanedLengthM"`

	AcceptanceTotal        int64   `json:"acceptanceTotal"`
	AcceptancePassCount    int64   `json:"acceptancePassCount"`
	AcceptancePassRate     float64 `json:"acceptancePassRate"`
	PendingAcceptanceCount int64   `json:"pendingAcceptanceCount"`
	PendingRectifyCount    int64   `json:"pendingRectifyCount"`
}

// Overview 按全局筛选（片区 + 时间范围）汇总各模块关键指标。
func (s *Service) Overview(ctx context.Context, filter Filter) (*Overview, error) {
	result := &Overview{
		SegmentByStatus: make(map[string]int64),
		TaskByStatus:    make(map[string]int64),
	}

	// ---------- 管段台账（快照口径：只受片区筛选影响） ----------
	segmentQuery := pipesegment.ListQuery{District: filter.District}
	total, err := s.segments.CountForDashboard(ctx, segmentQuery)
	if err != nil {
		return nil, err
	}
	result.SegmentTotal = total
	length, err := s.segments.SumLengthForDashboard(ctx, segmentQuery)
	if err != nil {
		return nil, err
	}
	result.SegmentTotalLengthM = num.Round2(length)

	uncleaned, err := s.segments.CountForDashboard(ctx, pipesegment.ListQuery{
		District:  filter.District,
		Uncleaned: true,
	})
	if err != nil {
		return nil, err
	}
	result.UncleanedSegmentCount = uncleaned

	segmentStatus, err := s.segments.CountGroupedForDashboard(ctx, segmentQuery, "status")
	if err != nil {
		return nil, err
	}
	result.SegmentByStatus = segmentStatus

	// ---------- 清淤任务（时间范围落在计划开始日期） ----------
	taskQuery := cleaningtask.ListQuery{
		District: filter.District,
		PlanFrom: filter.DateFrom,
		PlanTo:   filter.DateTo,
	}
	taskTotal, err := s.tasks.CountForDashboard(ctx, taskQuery)
	if err != nil {
		return nil, err
	}
	result.TaskTotal = taskTotal

	taskStatus, err := s.tasks.CountGroupedForDashboard(ctx, taskQuery, "status")
	if err != nil {
		return nil, err
	}
	result.TaskByStatus = taskStatus

	overdue, err := s.tasks.CountForDashboard(ctx, cleaningtask.ListQuery{
		District: filter.District,
		PlanFrom: filter.DateFrom,
		PlanTo:   filter.DateTo,
		Overdue:  true,
	})
	if err != nil {
		return nil, err
	}
	result.TaskOverdue = overdue

	pendingAcceptance, err := s.tasks.CountForDashboard(ctx, cleaningtask.ListQuery{
		District: filter.District,
		PlanFrom: filter.DateFrom,
		PlanTo:   filter.DateTo,
		Status:   cleaningtask.StatusCompleted,
	})
	if err != nil {
		return nil, err
	}
	result.PendingAcceptanceCount = pendingAcceptance

	// ---------- 清淤记录（时间范围落在清淤日期） ----------
	recordQuery := cleaningrecord.ListQuery{
		District: filter.District,
		DateFrom: filter.DateFrom,
		DateTo:   filter.DateTo,
	}
	recordTotal, err := s.records.CountForDashboard(ctx, recordQuery)
	if err != nil {
		return nil, err
	}
	result.RecordTotal = recordTotal

	recordSum, err := s.records.SumForDashboard(ctx, recordQuery)
	if err != nil {
		return nil, err
	}
	result.SludgeTotalM3 = num.Round2(recordSum.SludgeVolumeM3)
	result.CleanedLengthM = num.Round2(recordSum.CleanedLengthM)

	today := date.Today()
	monthStart := date.New(time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC))
	monthEnd := date.New(time.Date(today.Year(), today.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1))
	monthSum, err := s.records.SumForDashboard(ctx, cleaningrecord.ListQuery{
		District: filter.District,
		DateFrom: &monthStart,
		DateTo:   &monthEnd,
	})
	if err != nil {
		return nil, err
	}
	result.SludgeThisMonthM3 = num.Round2(monthSum.SludgeVolumeM3)

	// ---------- 验收记录（时间范围落在验收日期） ----------
	acceptanceQuery := acceptance.ListQuery{
		District: filter.District,
		DateFrom: filter.DateFrom,
		DateTo:   filter.DateTo,
	}
	acceptanceTotal, err := s.acceptances.CountForDashboard(ctx, acceptanceQuery)
	if err != nil {
		return nil, err
	}
	result.AcceptanceTotal = acceptanceTotal

	acceptanceByResult, err := s.acceptances.CountGroupedForDashboard(ctx, acceptanceQuery, "result")
	if err != nil {
		return nil, err
	}
	result.AcceptancePassCount = acceptanceByResult[acceptance.ResultPass]
	if acceptanceTotal > 0 {
		result.AcceptancePassRate = num.Round2(float64(result.AcceptancePassCount) / float64(acceptanceTotal) * 100)
	}

	pendingRectify, err := s.acceptances.CountForDashboard(ctx, acceptance.ListQuery{
		District:       filter.District,
		DateFrom:       filter.DateFrom,
		DateTo:         filter.DateTo,
		PendingRectify: true,
	})
	if err != nil {
		return nil, err
	}
	result.PendingRectifyCount = pendingRectify

	return result, nil
}

// DistrictStat 片区维度的统计。
type DistrictStat struct {
	District              string     `json:"district"`
	SegmentCount          int64      `json:"segmentCount"`
	SegmentLengthM        float64    `json:"segmentLengthM"`
	UncleanedSegmentCount int64      `json:"uncleanedSegmentCount"`
	LastCleanedAt         *date.Date `json:"lastCleanedAt"`
	TaskCount             int64      `json:"taskCount"`
	AcceptedTaskCount     int64      `json:"acceptedTaskCount"`
	SludgeVolumeM3        float64    `json:"sludgeVolumeM3"`
}

// DistrictStats 按片区统计管段规模与清淤成果。
//
// 管段列为台账快照（不受时间范围影响）；任务数与清淤量受全局时间范围影响，
// 其中管段按管段表去重计数，避免同一管段在多条任务 / 多条记录下被重复计数。
func (s *Service) DistrictStats(ctx context.Context, filter Filter) ([]DistrictStat, error) {
	type segmentRow struct {
		District              string
		SegmentCount          int64
		SegmentLengthM        float64
		UncleanedSegmentCount int64
		LastCleanedAt         *date.Date
	}
	segmentRows := make([]segmentRow, 0)
	err := s.db.WithContext(ctx).Table(refx.TablePipeSegments).
		Select(`district,
			COUNT(*) AS segment_count,
			COALESCE(SUM(length_m), 0) AS segment_length_m,
			COALESCE(SUM(CASE WHEN last_cleaned_at IS NULL THEN 1 ELSE 0 END), 0) AS uncleaned_segment_count,
			MAX(last_cleaned_at) AS last_cleaned_at`).
		Group("district").
		Order("district ASC").
		Scan(&segmentRows).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计片区管段失败", err)
	}

	// 任务行：与任务列表同口径（计划开始日期落在时间范围内），按管段所在片区分组。
	taskQuery := s.db.WithContext(ctx).Table(refx.TableCleaningTasks+" AS t").
		Select(`s.district AS district,
			COUNT(*) AS task_count,
			COALESCE(SUM(CASE WHEN t.status = ? THEN 1 ELSE 0 END), 0) AS accepted_task_count`,
			cleaningtask.StatusAccepted).
		Joins("INNER JOIN " + refx.TablePipeSegments + " AS s ON s.id = t.pipe_segment_id")
	if filter.DateFrom != nil {
		taskQuery = taskQuery.Where("t.plan_start_date >= ?", filter.DateFrom.Time)
	}
	if filter.DateTo != nil {
		taskQuery = taskQuery.Where("t.plan_start_date <= ?", filter.DateTo.Time)
	}
	type taskRow struct {
		District          string
		TaskCount         int64
		AcceptedTaskCount int64
	}
	taskRows := make([]taskRow, 0)
	if err := taskQuery.Group("s.district").Scan(&taskRows).Error; err != nil {
		return nil, httpx.WrapInternal("统计片区任务失败", err)
	}
	taskByDistrict := make(map[string]taskRow, len(taskRows))
	for _, row := range taskRows {
		taskByDistrict[row.District] = row
	}

	// 清淤量行：与清淤记录列表同口径（清淤日期落在时间范围内），按管段所在片区分组。
	recordQuery := s.db.WithContext(ctx).Table(refx.TableCleaningRecords + " AS r").
		Select(`s.district AS district, COALESCE(SUM(r.sludge_volume_m3), 0) AS sludge_volume_m3`).
		Joins("INNER JOIN " + refx.TableCleaningTasks + " AS t ON t.id = r.task_id").
		Joins("INNER JOIN " + refx.TablePipeSegments + " AS s ON s.id = t.pipe_segment_id")
	if filter.DateFrom != nil {
		recordQuery = recordQuery.Where("r.cleaned_at >= ?", filter.DateFrom.Time)
	}
	if filter.DateTo != nil {
		recordQuery = recordQuery.Where("r.cleaned_at <= ?", filter.DateTo.Time)
	}
	type sludgeRow struct {
		District       string
		SludgeVolumeM3 float64
	}
	sludgeRows := make([]sludgeRow, 0)
	if err := recordQuery.Group("s.district").Scan(&sludgeRows).Error; err != nil {
		return nil, httpx.WrapInternal("统计片区清淤量失败", err)
	}
	sludgeByDistrict := make(map[string]float64, len(sludgeRows))
	for _, row := range sludgeRows {
		sludgeByDistrict[row.District] = num.Round2(row.SludgeVolumeM3)
	}

	stats := make([]DistrictStat, 0, len(segmentRows))
	for _, row := range segmentRows {
		item := DistrictStat{
			District:              row.District,
			SegmentCount:          row.SegmentCount,
			SegmentLengthM:        num.Round2(row.SegmentLengthM),
			UncleanedSegmentCount: row.UncleanedSegmentCount,
			LastCleanedAt:         row.LastCleanedAt,
			SludgeVolumeM3:        sludgeByDistrict[row.District],
		}
		if task, ok := taskByDistrict[row.District]; ok {
			item.TaskCount = task.TaskCount
			item.AcceptedTaskCount = task.AcceptedTaskCount
		}
		stats = append(stats, item)
	}
	return stats, nil
}

// PendingAcceptanceItem 待验收任务。
type PendingAcceptanceItem struct {
	TaskID          uint       `json:"taskId"`
	Code            string     `json:"code"`
	Title           string     `json:"title"`
	SegmentCode     string     `json:"segmentCode"`
	SegmentName     string     `json:"segmentName"`
	SegmentDistrict string     `json:"segmentDistrict"`
	TeamName        string     `json:"teamName"`
	PlanEndDate     date.Date  `json:"planEndDate"`
	FinishedAt      *time.Time `json:"finishedAt"`
	RecordCount     int64      `json:"recordCount"`
	SludgeVolumeM3  float64    `json:"sludgeVolumeM3"`
	OverdueDays     int        `json:"overdueDays"`
}

// PendingAcceptance 待验收任务清单，按完工时间升序（先完工先验收）。
func (s *Service) PendingAcceptance(ctx context.Context, filter Filter, limit int) ([]PendingAcceptanceItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	query := s.db.WithContext(ctx).Table(refx.TableCleaningTasks+" AS t").
		Select(`t.id AS task_id, t.code, t.title, t.team_name, t.plan_end_date, t.finished_at,
			COALESCE(s.code, '') AS segment_code,
			COALESCE(s.name, '') AS segment_name,
			COALESCE(s.district, '') AS segment_district,
			COALESCE(r.record_count, 0) AS record_count,
			COALESCE(r.sludge_volume, 0) AS sludge_volume_m3`).
		Joins("LEFT JOIN "+refx.TablePipeSegments+" AS s ON s.id = t.pipe_segment_id").
		Joins(`LEFT JOIN (
			SELECT task_id, COUNT(*) AS record_count, SUM(sludge_volume_m3) AS sludge_volume
			FROM `+refx.TableCleaningRecords+` GROUP BY task_id
		) AS r ON r.task_id = t.id`).
		Where("t.status = ?", cleaningtask.StatusCompleted)
	if filter.District != "" {
		query = query.Where("s.district = ?", filter.District)
	}
	if filter.DateFrom != nil {
		query = query.Where("t.plan_start_date >= ?", filter.DateFrom.Time)
	}
	if filter.DateTo != nil {
		query = query.Where("t.plan_start_date <= ?", filter.DateTo.Time)
	}
	items := make([]PendingAcceptanceItem, 0, limit)
	if err := query.Order("t.finished_at ASC, t.id ASC").
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, httpx.WrapInternal("查询待验收任务失败", err)
	}

	today := date.Today()
	for i := range items {
		items[i].SludgeVolumeM3 = num.Round2(items[i].SludgeVolumeM3)
		if items[i].PlanEndDate.IsZero() {
			continue
		}
		if today.After(items[i].PlanEndDate) {
			items[i].OverdueDays = int(today.Time.Sub(items[i].PlanEndDate.Time).Hours() / 24)
		}
	}
	return items, nil
}

// RecentRecordItem 最近清淤记录。
type RecentRecordItem struct {
	RecordID       uint      `json:"recordId"`
	Code           string    `json:"code"`
	CleanedAt      date.Date `json:"cleanedAt"`
	TaskID         uint      `json:"taskId"`
	TaskCode       string    `json:"taskCode"`
	TaskTitle      string    `json:"taskTitle"`
	SegmentCode    string    `json:"segmentCode"`
	SegmentName    string    `json:"segmentName"`
	TeamName       string    `json:"teamName"`
	RecorderName   string    `json:"recorderName"`
	LengthM        float64   `json:"lengthM"`
	SludgeVolumeM3 float64   `json:"sludgeVolumeM3"`
}

// RecentRecords 最近录入的清淤记录（受片区筛选影响）。
func (s *Service) RecentRecords(ctx context.Context, filter Filter, limit int) ([]RecentRecordItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	query := s.db.WithContext(ctx).Table(refx.TableCleaningRecords + " AS r").
		Select(`r.id AS record_id, r.code, r.cleaned_at, r.length_m, r.sludge_volume_m3, r.recorder_name,
			t.id AS task_id, t.code AS task_code, t.title AS task_title, t.team_name,
			COALESCE(s.code, '') AS segment_code,
			COALESCE(s.name, '') AS segment_name`).
		Joins("INNER JOIN " + refx.TableCleaningTasks + " AS t ON t.id = r.task_id").
		Joins("LEFT JOIN " + refx.TablePipeSegments + " AS s ON s.id = t.pipe_segment_id")
	if filter.District != "" {
		query = query.Where("s.district = ?", filter.District)
	}
	if filter.DateFrom != nil {
		query = query.Where("r.cleaned_at >= ?", filter.DateFrom.Time)
	}
	if filter.DateTo != nil {
		query = query.Where("r.cleaned_at <= ?", filter.DateTo.Time)
	}
	items := make([]RecentRecordItem, 0, limit)
	if err := query.Order("r.cleaned_at DESC, r.id DESC").
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, httpx.WrapInternal("查询最近清淤记录失败", err)
	}
	return items, nil
}
