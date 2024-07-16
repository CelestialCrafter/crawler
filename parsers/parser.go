package parsers

import (
	"context"
	"net/url"

	pb "github.com/CelestialCrafter/crawler/protos"
)

type Parser interface {
	Fetch(data *pb.Document, ctx context.Context) error
	ParsePage(data *pb.Document, original *url.URL) error
}
