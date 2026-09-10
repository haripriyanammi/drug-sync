package normalize

import (
	"strings"

	"drugsync/fdaclient"
	drugpb "drugsync/proto"
)

// clean trims spaces and squashes runs of whitespace into a single space.
// "  Johnson &   Johnson " → "Johnson & Johnson"
func clean(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// first pulls item [0] out of a list, or gives back "" if the list is empty.
// This is what prevents the index-out-of-range panic.
func first(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return clean(list[0])
}

// ToDrug turns one messy openFDA record into one clean Drug.
// The bool is false when the record is unusable and should be skipped.
func ToDrug(r fdaclient.Result) (*drugpb.Drug, bool) {
	id := clean(r.ID)
	brand := first(r.OpenFDA.BrandName)
	generic := first(r.OpenFDA.GenericName)

	if id == "" || (brand == "" && generic == "") {
		return nil, false
	}

	return &drugpb.Drug{
		Id:            id,
		BrandName:     brand,
		GenericName:   generic,
		Manufacturer:  first(r.OpenFDA.Manufacturer),
		ProductNdc:    first(r.OpenFDA.ProductNDC),
		ProductType:   first(r.OpenFDA.ProductType),
		Route:         first(r.OpenFDA.Route),
		SubstanceName: first(r.OpenFDA.SubstanceName),
	}, true
}
