//go:build server

package zhasis

func InitCoreThings() {
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

	CreateHardCodedClass("string-id", "Some kind of string id used to identify things.", StringIDClassID, RootThingClassID)
	CreateHardCodedClass("serial-number", "A string containing mostly alphanumeric for identifying.", SerialNumberClassID, StringIDClassID)
	CreateHardCodedClass("ip-address", "An ip-address stored as a string. No port information.", IPAddressClassID, StringIDClassID)
	CreateHardCodedClass("ip4-address", "An ip4-address stored as 'A.B.C.D'.", IP4AddressClassID, IPAddressClassID)

	CreateHardCodedClass("hardware-uid", "An id uniquely identifying a hardware device", HardwareIDClassID, SerialNumberClassID)
	CreateHardCodedClass("auth-token", "A token used to perform secure communication", AuthTokenClassID, SerialNumberClassID)
	CreateHardCodedClass("vnc-ipaddress", "An ip-address to open a vnc-screen sharing connection to.", VNCIPAddressClassID, IPAddressClassID)
	CreateHardCodedClass("build-info", "A string showing details of when/how software was compiled.", BuildStringClassID, StructuredTextInfoClassID)

	CreateHardCodedClass("building", "Any kind of man-made building", BuildingClassID, RootThingClassID)
	CreateHardCodedClass("dwelling", "A building humans/animals live in", DwellingClassID, BuildingClassID)
	CreateHardCodedClass("house", "A stand-alone dwelling.", HouseClassID, DwellingClassID)
	CreateHardCodedClass("apartment", "A dwelling within a larger building, often multi-story.", ApartmentClassID, DwellingClassID)

	CreateHardCodedClass("place", "A physical place, abstract", PlaceClassID, RootThingClassID)
	CreateHardCodedClass("home", "A place people/animals live, within a dwelling", HomeClassID, PlaceClassID)
	// */
	CreateInstance(SerialNumberClassID, 1, "EFSS-033424-AXDFS")

	AddRelationToClass(HomeClassID, VerbWithin, DwellingClassID)
	AddRelationToClass(ApartmentClassID, VerbWithin, DwellingClassID)
}
