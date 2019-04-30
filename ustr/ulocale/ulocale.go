package ulocale

func GetMeter(plural bool, langCode string) string {
	if plural {
		return "Meters"
	}
	return "Meter"
}

func GetKiloMeter(plural bool, langCode string) string {
	if plural {
		return "Kilometers"
	}
	return "Kilometer"
}

func GetMile(plural bool, langCode string) string {
	if plural {
		return "miles"
	}
	return "mile"
}

func GetYard(plural bool, langCode string) string {
	if plural {
		return "yards"
	}
	return "yard"
}

func GetHour(plural bool, langCode string) string {
	if plural {
		return "hours"
	}
	return "hour"
}

func GetMinute(plural bool, langCode string) string {
	if plural {
		return "minutes"
	}
	return "minute"
}

func GetNorth(langCode string) string {
	switch langCode {
	case "no":
		return "nord"
	case "de":
		return "Norden"
	case "ja":
		return "北"
	default:
		return "North"
	}
}

func GetEast(langCode string) string {
	switch langCode {
	case "no":
		return "øst"
	case "de":
		return "Osten"
	case "ja":
		return "東"
	default:
		return "North"
	}
}

func GetSouth(langCode string) string {
	switch langCode {
	case "no":
		return "syd"
	case "de":
		return "Süden"
	case "ja":
		return "南"
	default:
		return "South"
	}
}

func GetWest(langCode string) string {
	switch langCode {
	case "no":
		return "vest"
	case "de":
		return "Westen"
	case "ja":
		return "西"
	default:
		return "West"
	}
}

func GetAnd(langCode string) string {
	switch langCode {
	case "no":
		return "og"
	case "de":
		return "und"
	case "ja":
		return "と"
	default:
		return "and"
	}
}
