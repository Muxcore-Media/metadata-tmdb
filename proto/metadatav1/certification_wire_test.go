package metadatav1

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// This deprecated copy is decoded by media-movies from the same metadata-tmdb
// server that encodes contracts-metadata messages, so the certification tags
// must match contracts-metadata exactly (movie 28/29, TV 33/34; ADR-0031 §2).
// The server-side tags are pinned in internal/certification_test.go.

func TestCertificationFieldNumbersMatchContract(t *testing.T) {
	cases := []struct {
		desc protoreflect.MessageDescriptor
		want map[protoreflect.Name]protoreflect.FieldNumber
	}{
		{(&GetMovieDetailsResponse{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
			"certification": 28, "certification_country": 29,
		}},
		{(&GetTVDetailsResponse{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
			"certification": 33, "certification_country": 34,
		}},
	}
	for _, c := range cases {
		for name, num := range c.want {
			fd := c.desc.Fields().ByName(name)
			if fd == nil {
				t.Errorf("%s: missing %s", c.desc.Name(), name)
				continue
			}
			if fd.Number() != num || fd.Kind() != protoreflect.StringKind || fd.Cardinality() != protoreflect.Optional {
				t.Errorf("%s.%s = tag %d %v %v, want tag %d singular string", c.desc.Name(), name, fd.Number(), fd.Kind(), fd.Cardinality(), num)
			}
		}
	}
}

func TestCertificationDecodesFromContractWire(t *testing.T) {
	enc := func(pairs ...any) []byte {
		var b []byte
		for i := 0; i < len(pairs); i += 2 {
			b = protowire.AppendTag(b, pairs[i].(protowire.Number), protowire.BytesType)
			b = protowire.AppendString(b, pairs[i+1].(string))
		}
		return b
	}
	var mv GetMovieDetailsResponse
	if err := proto.Unmarshal(enc(protowire.Number(2), "Fight Club", protowire.Number(28), "R", protowire.Number(29), "US"), &mv); err != nil {
		t.Fatal(err)
	}
	if mv.GetTitle() != "Fight Club" || mv.GetCertification() != "R" || mv.GetCertificationCountry() != "US" {
		t.Errorf("movie decoded = %q (%q,%q)", mv.GetTitle(), mv.GetCertification(), mv.GetCertificationCountry())
	}
	var tv GetTVDetailsResponse
	if err := proto.Unmarshal(enc(protowire.Number(2), "Breaking Bad", protowire.Number(33), "TV-MA", protowire.Number(34), "US"), &tv); err != nil {
		t.Fatal(err)
	}
	if tv.GetName() != "Breaking Bad" || tv.GetCertification() != "TV-MA" || tv.GetCertificationCountry() != "US" {
		t.Errorf("tv decoded = %q (%q,%q)", tv.GetName(), tv.GetCertification(), tv.GetCertificationCountry())
	}
}
