package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type InvoiceRequestOrder struct{ ent.Schema }

func (InvoiceRequestOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "invoice_request_orders"}}
}
func (InvoiceRequestOrder) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("invoice_request_id").Immutable(), field.Int64("order_id").Immutable(),
		field.String("order_no").MaxLen(64).Immutable(), field.String("order_type").MaxLen(20).Immutable(),
		field.String("name").MaxLen(200).Immutable(), field.Float("amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).Immutable(),
		field.Time("released_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
func (InvoiceRequestOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("invoice_request_id"),
		index.Fields("order_id").Unique().Annotations(entsql.IndexWhere("released_at IS NULL")),
	}
}
