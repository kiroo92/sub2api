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

type InviteBinding struct {
	ent.Schema
}

func (InviteBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invite_bindings"},
	}
}

func (InviteBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("inviter_user_id"),
		field.Int64("invitee_user_id"),
		field.Int64("invite_code_id"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (InviteBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("inviter", User.Type).Ref("invited_users").Field("inviter_user_id").Unique().Required(),
		edge.From("invitee", User.Type).Ref("invite_binding").Field("invitee_user_id").Unique().Required(),
		edge.From("invite_code", InviteCode.Type).Ref("bindings").Field("invite_code_id").Unique().Required(),
	}
}

func (InviteBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("invitee_user_id").Unique(),
		index.Fields("inviter_user_id"),
		index.Fields("invite_code_id"),
	}
}
