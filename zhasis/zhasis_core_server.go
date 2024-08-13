//go:build server

package zhasis

import "github.com/torlangballe/zutil/zlog"

func InitCoreThings() {
	const userID = 1
	// /*
	CreateHardCodedClass("state", "A general state of something", StateClassID, RootThingClassID)
	CreateHardCodedClass("bool-state", "A boolean state, which is 0, 1, or '' for undef", BoolStateClassID, StateClassID)
	CreateHardCodedClass("active", "a state of being activated", ActivatedClassID, BoolStateClassID)
	CreateHardCodedClass("struct-text", "A text string with structured information", StructuredTextInfoClassID, RootThingClassID)

	CreateHardCodedClass("measurement", "A general metric", MetricClassID, RootThingClassID)
	CreateHardCodedClass("duration", "a duration of time", DurationClassID, MetricClassID)
	CreateHardCodedClass("duration-secs", "a duration of time in seconds", DurationSecsClassID, DurationClassID)
	CreateHardCodedClass("timestamp", "a point in time", TimeStampClassID, MetricClassID)
	CreateHardCodedClass("timestamp-rfc3339", "a timestamp stored as an RFC3339 string with nano precision and time zone", TimeStampRFCClassID, MetricClassID)
	CreateHardCodedClass("pinged", "a time something was pinged, or contacted", PingedTimeClassID, TimeStampRFCClassID)
	CreateHardCodedClass("index", "A value that identifies a sub-components order in parent hierarchy", IndexClassID, MetricClassID)
	CreateHardCodedClass("numeric-offset-index", "A linear index into an order list. Non-integers can refer to halfway etc", NumericOffsetIndexClassID, MetricClassID)

	CreateHardCodedClass("string-id", "Some kind of string id used to identify things.", StringIDClassID, RootThingClassID)
	CreateHardCodedClass("serial-number", "A string containing mostly alphanumeric for identifying.", SerialNumberClassID, StringIDClassID)
	CreateHardCodedClass("ip-address", "An ip-address stored as a string. No port information.", IPAddressClassID, StringIDClassID)
	CreateHardCodedClass("ip4-address", "An ip4-address stored as 'A.B.C.D'.", IP4AddressClassID, IPAddressClassID)

	CreateHardCodedClass("hardware-uid", "An id uniquely identifying a hardware device", HardwareIDClassID, SerialNumberClassID)
	CreateHardCodedClass("auth-token", "A token used to perform secure communication", AuthTokenClassID, SerialNumberClassID)
	CreateHardCodedClass("vnc-ipaddress", "An ip-address to open a vnc-screen sharing connection to.", VNCIPAddressClassID, IPAddressClassID)
	CreateHardCodedClass("build-info", "A string showing details of when/how software was compiled.", BuildStringClassID, StructuredTextInfoClassID)

	CreateHardCodedClass("physical-place", "A physical place, a structure or part of bigger structure", PhysicalPlaceClassID, RootThingClassID)
	CreateHardCodedClass("shelter", "something built to take shelter in/under", ShelterClassID, RootThingClassID)
	CreateHardCodedClass("building", "Any kind of man-made building", BuildingClassID, ShelterClassID)
	CreateHardCodedClass("dwelling", "A building humans/animals live in", DwellingClassID, PhysicalPlaceClassID)
	AddRelationToClass(DwellingClassID, VerbWithin, ShelterClassID, nil) // A dwelling is within a shelter, might not be whole thing

	CreateHardCodedClass("house", "A stand-alone dwelling.", HouseClassID, DwellingClassID)
	CreateHardCodedClass("apartment", "A home within a larger building, often multi-story.", ApartmentClassID, DwellingClassID)

	CreateHardCodedClass("floor", "The physical entire floor of a building", FloorClassID, NumericOffsetIndexClassID)
	CreateHardCodedClass("floor-index", "The floor index in a building (not actual tiled floor etc). Can be 1.5 for in stairwell. 0-indexed, not same as FloorIDClassID", FloorIndexClassID, NumericOffsetIndexClassID)
	CreateHardCodedClass("floor-id", "The floor identifier used identify a dwelling in a multi-story building", FloorIdentifierClassID, StructuredTextInfoClassID)
	AddRelationToClass(FloorClassID, VerbPartOf, BuildingClassID, nil)
	AddRelationToClass(FloorIdentifierClassID, VerbAttributeOf, FloorClassID, nil)
	AddRelationToClass(FloorIndexClassID, VerbAttributeOf, FloorClassID, nil)

	CreateHardCodedClass("space", "An defined area.", SpaceAreaClassID, RootThingClassID)
	CreateHardCodedClass("room", "A compartment of a dwelling.", RoomClassID, SpaceAreaClassID)
	AddRelationToClass(RoomClassID, VerbPartOf, DwellingClassID, nil)
	AddRelationToClass(FloorIndexClassID, VerbAttributeOf, RoomClassID, nil)
	AddRelationToClass(ApartmentClassID, VerbOn, FloorClassID, nil)

	CreateHardCodedClass("abstract-place", "An abstract place", AbstractPlaceClassID, RootThingClassID)
	CreateHardCodedClass("residence", "A place on lives, within a dwelling", ResidenceClassID, AbstractPlaceClassID)
	CreateHardCodedClass("home", "A place people/animals live, within a dwelling", HomeClassID, ResidenceClassID)
	AddRelationToClass(RoomClassID, VerbPartOf, DwellingClassID, nil)
	residenceWithinDwellingRel, _ := AddRelationToClass(ResidenceClassID, VerbWithin, DwellingClassID, nil) // This links abstract residence to inside physical dwelling

	CreateHardCodedClass("residence identifier", "The number of a residence, usually an index in street with 4b etc, or a name", ResidenceIdentifierClassID, StructuredTextInfoClassID)
	ResidencesIdentifierAttributeID, _ = AddRelationToClass(ResidenceIdentifierClassID, VerbAttributeOf, ResidenceClassID, nil)

	zlog.Info("ResidencesIdentifierAttributeID", ResidencesIdentifierAttributeID)
	CreateHardCodedClass("way-noun", "A road, track, or path for traveling along", WayNounClassID, RootThingClassID)
	CreateHardCodedClass("road", "a wide way leading from one place to another, often with surface/for vehicles", RoadClassID, WayNounClassID)
	CreateHardCodedClass("street", "a public road in a city, town, or village, typically with houses and buildings on one or both sides", StreetClassID, RoadClassID)
	CreateHardCodedClass("path-on-ground", "a public road in a city, town, or village, typically with houses and buildings on one or both sides", PathOnGroundClassID, WayNounClassID)

	// */
	a2, _ := CreateConstant("2A")
	homeNumber, _ := CreateInstance(ResidenceIdentifierClassID, userID, a2)
	fhg2, _ := CreateConstant("FHG2")
	homeID, _ := CreateInstance(HomeClassID, userID, fhg2)
	// zlog.Info("HomeID:", homeID, err)

	AddRelationToClass(ResidenceClassID, VerbWithin, DwellingClassID, &residenceWithinDwellingRel)
	AddRelationToClass(HomeClassID, VerbWithin, DwellingClassID, nil)

	AddValueRelationToInstance(homeNumber, ResidencesIdentifierAttributeID, homeID, 1)
}
