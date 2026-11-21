package billing_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelDiscountUsesFirstEnabledGroup(t *testing.T) {
	original := billingSetting.ModelDiscountGroups
	t.Cleanup(func() { billingSetting.ModelDiscountGroups = original })

	billingSetting.ModelDiscountGroups = []ModelDiscountGroup{
		{Name: "disabled", Discount: 0.1, Models: []string{"a"}, Enabled: false},
		{Name: "first", Discount: 0.3, Models: []string{"a", "b"}, Enabled: true},
		{Name: "second", Discount: 0.5, Models: []string{"a"}, Enabled: true},
		{Name: "invalid", Discount: 0, Models: []string{"c"}, Enabled: true},
	}

	discount, group, ok := GetModelDiscount("a")
	require.True(t, ok)
	assert.Equal(t, 0.3, discount)
	assert.Equal(t, "first", group)

	_, _, ok = GetModelDiscount("missing")
	assert.False(t, ok)

	_, _, ok = GetModelDiscount("c")
	assert.False(t, ok, "invalid discount ratios must be ignored")
}

func TestValidateModelDiscountGroupsJSON(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid enabled group",
			raw:  `[{"name":"A","discount":0.5,"models":["m1","m2"],"enabled":true}]`,
		},
		{
			name: "duplicate model in disabled group is allowed",
			raw:  `[{"name":"A","discount":0.5,"models":["m1"],"enabled":true},{"name":"B","discount":0.5,"models":["m1"],"enabled":false}]`,
		},
		{
			name:    "duplicate model across enabled groups",
			raw:     `[{"name":"A","discount":0.5,"models":["m1"],"enabled":true},{"name":"B","discount":0.5,"models":["m1"],"enabled":true}]`,
			wantErr: "appears in multiple discount groups",
		},
		{
			name:    "zero discount",
			raw:     `[{"name":"A","discount":0,"models":["m1"],"enabled":true}]`,
			wantErr: "greater than 0",
		},
		{
			name:    "discount above one",
			raw:     `[{"name":"A","discount":1.2,"models":["m1"],"enabled":true}]`,
			wantErr: "greater than 0",
		},
		{
			name:    "empty name",
			raw:     `[{"name":" ","discount":0.5,"models":["m1"],"enabled":true}]`,
			wantErr: "name is required",
		},
		{
			name:    "empty model list",
			raw:     `[{"name":"A","discount":0.5,"models":[],"enabled":true}]`,
			wantErr: "at least one model",
		},
		{
			name:    "blank model name",
			raw:     `[{"name":"A","discount":0.5,"models":[" "],"enabled":true}]`,
			wantErr: "must not be empty",
		},
		{
			name:    "invalid json",
			raw:     `{`,
			wantErr: "Invalid discount group configuration",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateModelDiscountGroupsJSON(tc.raw)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
