package migrations

import (
	"strings"
	"testing"
)

func TestPackageMigrationIsIndependent(t *testing.T) {
	data, err := FS.ReadFile("240_packages.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(data))
	for _, old := range []string{"user_subscriptions", "subscription_plans", "subscriptions_user_id_group_id"} {
		if strings.Contains(sql, old) {
			t.Fatalf("package migration modifies legacy source %q", old)
		}
	}
	for _, contract := range []string{"unique(group_buy_id,user_id)", "unique(package_id,period_index)", "references package_orders(order_id)", "routing_mode <> 'all_packages' or group_id is null"} {
		if !strings.Contains(sql, contract) {
			t.Fatalf("missing contract: %s", contract)
		}
	}
}
