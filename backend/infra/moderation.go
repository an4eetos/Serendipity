package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/internal/moderation/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/moderation/adapters/middleware"
	"github.com/an4eetos/serendipity/backend/internal/moderation/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
	"github.com/an4eetos/serendipity/backend/internal/moderation/usecase"
)

// Moderation acts on readers' content through the quote and discussion
// moderation ports. RestoreTarget's word cap is the quote module's
// CoverageCap: Fx modules share one graph.
var Moderation = fx.Module("moderation",
	fx.Provide(
		fx.Annotate(postgres.NewReportRepository, fx.As(new(ports.ReportRepository))),
		fx.Annotate(postgres.NewTakedownRepository, fx.As(new(ports.TakedownRepository))),
		fx.Annotate(postgres.NewAdminRepository, fx.As(new(ports.AdminRepository))),
		usecase.NewCheckAdmin,
		usecase.NewCreateReport,
		usecase.NewListOpenReports,
		usecase.NewHideTarget,
		usecase.NewDismissReports,
		usecase.NewListTakedowns,
		usecase.NewRestoreTarget,
		middleware.NewAdmin,
		asRoutes(handlers.New),
	),
)
