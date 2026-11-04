package billing_setting

import (
	"math"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ModelDiscountGroupsOption is the DB option key of the discount group list.
const ModelDiscountGroupsOption = "billing_setting.model_discount_groups"

// ModelDiscountGroup applies one promotional discount to a hand-picked set of
// models. Only enabled groups take effect; the first enabled group that lists a
// model wins. Discount is a multiplier: 0.3 means the model is billed at 30% of
// its configured price.
type ModelDiscountGroup struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Discount float64  `json:"discount"`
	Models   []string `json:"models"`
	Enabled  bool     `json:"enabled"`
}

// GetModelDiscount returns the discount ratio and group name for a model.
func GetModelDiscount(model string) (float64, string, bool) {
	if model == "" {
		return 0, "", false
	}
	for i := range billingSetting.ModelDiscountGroups {
		group := &billingSetting.ModelDiscountGroups[i]
		if !group.Enabled || !validDiscount(group.Discount) {
			continue
		}
		if slices.Contains(group.Models, model) {
			return group.Discount, group.Name, true
		}
	}
	return 0, "", false
}

// GetModelDiscountGroups returns a copy of the configured groups for display.
func GetModelDiscountGroups() []ModelDiscountGroup {
	groups := make([]ModelDiscountGroup, len(billingSetting.ModelDiscountGroups))
	for i, group := range billingSetting.ModelDiscountGroups {
		groups[i] = group
		groups[i].Models = slices.Clone(group.Models)
	}
	return groups
}

func validDiscount(discount float64) bool {
	return discount > 0 && discount <= 1 && !math.IsNaN(discount) && !math.IsInf(discount, 0)
}

// ValidateModelDiscountGroupsJSON validates a candidate option value before it
// is persisted. A model may belong to only one enabled group.
func ValidateModelDiscountGroupsJSON(raw string) error {
	var groups []ModelDiscountGroup
	if err := common.UnmarshalJsonStr(raw, &groups); err != nil {
		return common.NewMessage("Invalid discount group configuration")
	}
	owner := make(map[string]string)
	for index, group := range groups {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			return common.NewMessage("Discount group name is required", map[string]any{"index": index + 1})
		}
		if !validDiscount(group.Discount) {
			return common.NewMessage("Discount must be greater than 0 and at most 1", map[string]any{"group": name})
		}
		if len(group.Models) == 0 {
			return common.NewMessage("Discount group must include at least one model", map[string]any{"group": name})
		}
		if !group.Enabled {
			continue
		}
		for _, model := range group.Models {
			model = strings.TrimSpace(model)
			if model == "" {
				return common.NewMessage("Discount group model must not be empty", map[string]any{"group": name})
			}
			if previous, exists := owner[model]; exists {
				return common.NewMessage("Model {{model}} appears in multiple discount groups", map[string]any{"model": model, "group": previous})
			}
			owner[model] = name
		}
	}
	return nil
}
