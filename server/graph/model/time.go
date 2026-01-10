package model

import (
	"time"

	"github.com/99designs/gqlgen/graphql"
)

type Time time.Time

func MarshalTime(t Time) graphql.Marshaler {
	return graphql.MarshalTime(time.Time(t))
}

func UnmarshalTime(v any) (Time, error) {
	t, err := graphql.UnmarshalTime(v)

	return Time(t), err
}
