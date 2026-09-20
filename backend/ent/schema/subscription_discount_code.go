package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// SubscriptionDiscountCode changes a subscription purchase price, not its entitlement.
type SubscriptionDiscountCode struct{ ent.Schema }

func (SubscriptionDiscountCode) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subscription_discount_codes"}}
}

func (SubscriptionDiscountCode) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty().Unique().Immutable(),
		field.String("discount_type").MaxLen(20),
		field.Float("discount_value").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.JSON("plan_ids", []int64{}).Default([]int64{}),
		field.Bool("enabled").Default(true),
		field.Time("expires_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int("max_uses").Default(0).NonNegative(),
		field.Int("per_user_limit").Default(1).NonNegative(),
		field.Time("created_at").Default(time.Now).Immutable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
