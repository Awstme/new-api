package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/model"
)

func excludeAdminUserIDs(c *gin.Context) ([]int, error) {
	enabled, _ := strconv.ParseBool(c.Query("exclude_admins"))
	if !enabled {
		return nil, nil
	}
	return model.GetAdminUserIDs()
}
