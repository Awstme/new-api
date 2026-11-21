package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// setOperatingRouter 挂载经营分析接口。它只读取现有业务表并维护一张独立的投入表，
// 因此不接入计费、结算或钱包扣减路径。
func setOperatingRouter(apiRouter *gin.RouterGroup) {
	operatingRoute := apiRouter.Group("/operating")
	operatingRoute.Use(middleware.AdminAuth())
	{
		operatingRoute.GET("/summary", controller.GetOperatingSummary)
		operatingRoute.GET("/flow", controller.GetOperatingFlow)
		operatingRoute.GET("/expenses", controller.GetOperatingExpenses)
		operatingRoute.POST("/expenses", controller.CreateOperatingExpense)
		operatingRoute.DELETE("/expenses/:id", controller.DeleteOperatingExpense)
	}
}
