package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/modules/acceptance"
	"github.com/drainage/desilting/internal/modules/cleaningrecord"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/pipesegment"
)

// Register 注册看板路由，并接入各业务模块的统计能力。
func Register(
	router fiber.Router,
	db *gorm.DB,
	segments *pipesegment.Service,
	tasks *cleaningtask.Service,
	records *cleaningrecord.Service,
	acceptances *acceptance.Service,
) *Service {
	svc := NewService(db, segments, tasks, records, acceptances)
	handler := NewHandler(svc)

	group := router.Group("/dashboard")
	group.Get("/overview", handler.Overview)
	group.Get("/district-stats", handler.DistrictStats)
	group.Get("/pending-acceptance", handler.PendingAcceptance)
	group.Get("/recent-records", handler.RecentRecords)

	return svc
}
