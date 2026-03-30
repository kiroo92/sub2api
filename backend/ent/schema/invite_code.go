package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type InviteCode struct {
	ent.Schema
}

func (InviteCode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invite_codes"},
	}
}

func (InviteCode) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("code").MaxLen(32).NotEmpty(),
		field.Bool("active").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (InviteCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("invite_codes").Field("user_id").Unique().Required(),
		edge.To("bindings", InviteBinding.Type),
	}
}

func (InviteCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("code").Unique(),
	}
}
