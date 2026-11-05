package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendModelDiscountInfo(t *testing.T) {
	other := model.NewLogOther()
	appendModelDiscountInfo(other, hosttypes.PriceData{ModelDiscountRatio: 0.3, ModelDiscountGroup: "3折"})

	var decoded map[string]any
	require.NoError(t, common.UnmarshalJsonStr(other.JSONString(), &decoded))
	assert.Equal(t, 0.3, decoded["model_discount_ratio"])
	assert.Equal(t, "3折", decoded["model_discount_group"])

	plain := model.NewLogOther()
	appendModelDiscountInfo(plain, hosttypes.PriceData{})
	var plainDecoded map[string]any
	require.NoError(t, common.UnmarshalJsonStr(plain.JSONString(), &plainDecoded))
	assert.NotContains(t, plainDecoded, "model_discount_ratio")
	assert.NotContains(t, plainDecoded, "model_discount_group")
}
