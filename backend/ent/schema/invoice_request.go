package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"time"
)

// InvoiceRequest freezes a whole-order invoice and its separately paid service fee.
type InvoiceRequest struct{ ent.Schema }

func (InvoiceRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "invoice_requests"}}
}
func (InvoiceRequest) Fields() []ent.Field {
	money := map[string]string{dialect.Postgres: "decimal(20,2)"}
	timestamp := map[string]string{dialect.Postgres: "timestamptz"}
	return []ent.Field{
		field.Int64("user_id").Immutable(),
		field.String("status").MaxLen(24).Default("awaiting_payment"),
		field.String("tax_id").MaxLen(64), field.String("title").MaxLen(200),
		field.String("email").MaxLen(254), field.String("remarks").MaxLen(1000).Default(""),
		field.String("currency").MaxLen(3).Default("CNY"),
		field.Float("base_amount").SchemaType(money), field.Float("service_fee").SchemaType(money),
		field.Float("total_amount").SchemaType(money), field.Float("net_amount").SchemaType(money), field.Float("tax_amount").SchemaType(money),
		field.String("item_name").MaxLen(200), field.Float("tax_rate").SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),
		field.String("fee_type").MaxLen(20), field.Float("fee_value").SchemaType(money),
		field.Float("fee_upper_amount").Optional().Nillable().SchemaType(money),
		field.String("operation_key").MaxLen(64).Unique().Immutable(), field.String("fingerprint").MaxLen(64).Immutable(),
		field.Time("expires_at").SchemaType(timestamp), field.Time("submitted_at").Optional().Nillable().SchemaType(timestamp),
		field.Time("issued_at").Optional().Nillable().SchemaType(timestamp), field.Int64("issued_by").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable().SchemaType(timestamp),
	}
}
func (InvoiceRequest) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id", "created_at"), index.Fields("status", "expires_at")}
}
