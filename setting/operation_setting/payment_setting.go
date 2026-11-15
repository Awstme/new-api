package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type PaymentSetting struct {
	// AmountOptions 是站内货币的实付金额，支持两位小数，例如 9.9 表示充值 9.9 元。
	AmountOptions []float64 `json:"amount_options"`
	// AmountDiscount 的键是站内货币的实付金额，例如 "100": 0.9 表示 100 元充值享受 9 折优惠。
	AmountDiscount map[string]float64 `json:"amount_discount"`

	ComplianceConfirmed    bool   `json:"compliance_confirmed"`
	ComplianceTermsVersion string `json:"compliance_terms_version"`
	ComplianceConfirmedAt  int64  `json:"compliance_confirmed_at"`
	ComplianceConfirmedBy  int    `json:"compliance_confirmed_by"`
	ComplianceConfirmedIP  string `json:"compliance_confirmed_ip"`
}

const CurrentComplianceTermsVersion = "v1"

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:  []float64{10, 20, 50, 100, 200, 500},
	AmountDiscount: map[string]float64{},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

func IsPaymentComplianceConfirmed() bool {
	return paymentSetting.ComplianceConfirmed &&
		paymentSetting.ComplianceTermsVersion == CurrentComplianceTermsVersion
}
