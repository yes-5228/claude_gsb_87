package dashboard

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/shared/date"
)

// Handler 看板 HTTP 接口。
type Handler struct {
	svc *Service
}

// NewHandler 构造处理器。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// parseFilter 解析看板的全局筛选条件（片区 + 时间范围）。
func parseFilter(c *fiber.Ctx) (Filter, error) {
	filter := Filter{
		District: httpx.TrimmedQuery(c, "district"),
	}
	rawFrom := httpx.TrimmedQuery(c, "dateFrom")
	if rawFrom != "" {
		from, err := date.Parse(rawFrom)
		if err != nil {
			return Filter{}, httpx.BadRequest(fmt.Sprintf("时间范围起格式不正确，应为 YYYY-MM-DD"))
		}
		filter.DateFrom = &from
	}
	rawTo := httpx.TrimmedQuery(c, "dateTo")
	if rawTo != "" {
		to, err := date.Parse(rawTo)
		if err != nil {
			return Filter{}, httpx.BadRequest(fmt.Sprintf("时间范围止格式不正确，应为 YYYY-MM-DD"))
		}
		filter.DateTo = &to
	}
	if filter.DateFrom != nil && filter.DateTo != nil && filter.DateFrom.After(*filter.DateTo) {
		return Filter{}, httpx.BadRequest("时间范围起不能晚于时间范围止")
	}
	return filter, nil
}

// Overview 总览指标。
func (h *Handler) Overview(c *fiber.Ctx) error {
	filter, err := parseFilter(c)
	if err != nil {
		return err
	}
	overview, err := h.svc.Overview(c.UserContext(), filter)
	if err != nil {
		return err
	}
	return httpx.OK(c, overview)
}

// DistrictStats 片区统计。
func (h *Handler) DistrictStats(c *fiber.Ctx) error {
	filter, err := parseFilter(c)
	if err != nil {
		return err
	}
	stats, err := h.svc.DistrictStats(c.UserContext(), filter)
	if err != nil {
		return err
	}
	return httpx.OK(c, stats)
}

// PendingAcceptance 待验收任务清单。
func (h *Handler) PendingAcceptance(c *fiber.Ctx) error {
	filter, err := parseFilter(c)
	if err != nil {
		return err
	}
	items, err := h.svc.PendingAcceptance(c.UserContext(), filter, c.QueryInt("limit", 10))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

// RecentRecords 最近清淤记录。
func (h *Handler) RecentRecords(c *fiber.Ctx) error {
	filter, err := parseFilter(c)
	if err != nil {
		return err
	}
	items, err := h.svc.RecentRecords(c.UserContext(), filter, c.QueryInt("limit", 10))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}
