package google

import (
//	"github.com/torlangballe/zutil/places"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/uhttp"
	"github.com/torlangballe/zutil/ztime"

	//	speech "cloud.google.com/go/speech/apiv1"
	//	"context"
	"errors"
	"fmt"

	"github.com/kaneshin/pigeon"
	"github.com/kaneshin/pigeon/credentials"
	"google.golang.org/api/vision/v1"

	//	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// https://developers.google.com/maps/documentation/geocoding/#Types

type gLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}
type gGeometry struct {
	Location gLocation `json:"location"`
}
type gResults struct {
	Geometry gGeometry `json:"geometry"`
}
type gJSON struct {
	Results []gResults `json:"results"`
	Status  string     `json:"status"`
}

func GeocodePlaceName(w io.Writer, placename string) (pos zgeo.FPoint, err error) {
	var geo gJSON
	surl := "http://maps.googleapis.com/maps/api/geocode/json"
	u, err := url.Parse(surl)
	if err != nil {
		return
	}
	q := u.Query()
	//	q.Set("key", os.Getenv("GOOGLE_MAPS_KEY"))
	q.Set("address", placename)
	q.Set("sensor", "false")
	u.RawQuery = q.Encode()
	surl = u.String()
	fmt.Println("Google GeocodePlaceName:", surl)
	_, err = uhttp.UnmarshalFromJSONFromURL(surl, &geo, false, "", "")
	if err != nil {
		return
	}
	//	fmt.Fprintln(w, "Geo:", geo, surl)
	if geo.Status == "" {
		err = errors.New("Couldn't parse json from url: " + surl)
	}
	if geo.Status != "OK" {
		err = errors.New("Error geocoding from google: " + geo.Status)
	}
	if len(geo.Results) > 0 {
		pos.X = geo.Results[0].Geometry.Location.Lng
		pos.Y = geo.Results[0].Geometry.Location.Lat
		//		fmt.Println(geo.Results)
	}
	return
}

type tzJSON struct {
	TimeZoneId string `json:"timeZoneId"`
}

func GetTimeZoneFromLocation(pos zgeo.FPoint) (string, error) {
	var tzStruct tzJSON
	u, err := url.Parse("https://maps.googleapis.com/maps/api/timezone/json")
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	q := u.Query()
	//	q.Set("key", os.Getenv("GOOGLE_MAPS_KEY"))
	q.Set("location", fmt.Sprintf("%f,%f", pos.Y, pos.X))
	q.Set("timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	q.Set("sensor", "false")
	u.RawQuery = q.Encode()
	surl := u.String()
	_, err = uhttp.UnmarshalFromJSONFromURL(surl, &tzStruct, false, "", "")
	if err == nil && tzStruct.TimeZoneId == "" {
		err = errors.New("Couldn't parse json from url: " + surl)
	}
	tzname := utime.ReplaceOldUnixTimeZoneNamesWithNew(tzStruct.TimeZoneId)
	return tzname, err
}

func GetGeocodeURL(lang string, x, y float64) string {
	surl := "http://maps.googleapis.com/maps/api/geocode/json?latlng=%f,%f&sensor=true&language=%s"
	return fmt.Sprintf(surl, y, x, lang)
}

func GeocodeLocation(lang string, x, y float64) (gCode Geocode, err error) {
	// limit per day
	surl := GetGeocodeURL(lang, x, y)
	resp, err := http.Get(surl) // note swap!
	fmt.Println("Google GeocodeLocation:", surl)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &gCode)
	if err != nil {
		return
	}

	if gCode.Status != "OK" {
		err = fmt.Errorf(`zgeo: error response from Google "%s"`, gCode.Status)
		return
	}
	return
}

func GeocodeLocationToNameMap(lang string, x, y float64) (names map[string]string, err error) {
	gCode, err := GeocodeLocation(lang, x, y)
	if err != nil {
		return
	}
	names = getMapInfo(&gCode)
	return
}

func getMapInfo(gCode *Geocode) map[string]string {
	names := make(map[string]string)
	mostDetailed := 0
	mostDetailedIndex := 0

	for i, v := range gCode.Results {
		if num := len(v.Components); num > mostDetailed {
			mostDetailedIndex = i
			mostDetailed = num
		}
	}
	if mostDetailed == 0 {
		return names
	}
	for _, com := range gCode.Results[mostDetailedIndex].Components {
		for _, t := range com.Types {
			switch t {
			case "street_number":
				names[places.KStreetNoKey] = com.LongName
			case "route":
				names[places.KRouteKey] = com.LongName
			case "sublocality":
				names[places.KSubLocalityKey] = com.LongName
			case "locality":
				names[places.KLocalityKey] = com.LongName
			case "administrative_area_level_2":
				names[places.KAdminAreaKey] = com.LongName
			case "administrative_area_level_1":
				names[places.KAdminAreaSuperKey] = com.LongName
			case "postal_town":
				names[places.KPostalTownKey] = com.LongName
			case "country":
				names[places.KCountryCodeKey] = strings.ToLower(com.ShortName)
			}
		}
	}

	return names
}

func GetMapImageUrlFromLocation(pos zgeo.FPoint, size zgeo.FSize, myLocation zgeo.FPoint, langCode string) (url string, err error) {
	//	personIcon := "http://maps.google.com/mapfiles/kml/shapes/man.png"
	args := map[string]string{
		"size":     fmt.Sprintf("%gx%g", size.W, size.H),
		"language": langCode,
		"key":      "AIzaSyDLL1WYXwKFcibBR254seV5UuRIO20petQ",
		//		"markers":  fmt.Sprintf("color:blue|icon:%s|%g,%g", personIcon, pos.Y, pos.X),
		"markers": fmt.Sprintf("color:yellow|%g,%g", pos.Y, pos.X),
	}
	str := "https://maps.googleapis.com/maps/api/staticmap"
	url, err = uhttp.MakeURLWithArgs(str, args)
	return
}

func GetMapUrlToLocation(pos zgeo.FPoint, zoom int) string {
	return fmt.Sprintf("http://maps.google.com/maps?q=%f,%f&z=%d", pos.Y, pos.X, zoom)
}

func GetStaticMapUrlToLocations(positions []zgeo.FPoint) string {
	prefix := "http://maps.google.com/maps/api/staticmap"
	var postfix string
	for _, p := range positions {
		postfix += fmt.Sprintf("&markers=color:red|label:S|%g,%g", p.Y, p.X)
	}
	args := map[string]string{
		//		"center": ""
		"size": "1024x1024",
	}
	surl, _ := uhttp.MakeURLWithArgs(prefix, args)

	return surl + postfix

}

var googleCreds *credentials.Credentials = nil

const (
	KUnknown      = "UNKNOWN"
	KVeryUnlikely = "VERY_UNLIKELY"
	KUnlikely     = "UNLIKELY"
	KPossible     = "POSSIBLE"
	KLikely       = "LIKELY"
	KVeryLikely   = "VERY_LIKELY"
)

const (
	KAdult    = "adult"
	KMedical  = "medical"
	KSpoof    = "spoof"
	KViolence = "violence"
)

type ImageResult struct {
	LabelAnnotations []struct {
		Description string  `json:"description"`
		Score       float32 `json:"score"`
	} `json:"labelAnnotations"`
	LandmarkAnnotations []struct {
		Description string  `json:"description"`
		Score       float32 `json:"score"`
	} `json:"landmarkAnnotations"`
	SafeSearchAnnotation map[string]string `json:"safeSearchAnnotation"`
}

func AnnotateImage(imageUrl string, features ...*vision.Feature) (result ImageResult, err error) {
	if googleCreds == nil {
		googleCreds = credentials.NewApplicationCredentials("")
	}
	if googleCreds == nil {
		return
	}
	httpClient := http.DefaultClient
	httpClient.Timeout = time.Second * 30
	config := pigeon.NewConfig().WithHTTPClient(httpClient).WithCredentials(googleCreds)
	client, err := pigeon.New(config)
	if err != nil {
		return
	}

	batch, err := client.NewBatchAnnotateImageRequest([]string{imageUrl}, features...)
	if err != nil {
		return
	}
	/*
		for i := range batch.Requests {
			batch.Requests[i].ImageContext = &vision.ImageContext{}
			batch.Requests[i].ImageContext.LanguageHints = []string{"no"}
		}
	*/
	res, err := client.ImagesService().Annotate(batch).Do()
	if err != nil {
		return
	}

	bjson, _ := res.MarshalJSON()

	var results struct {
		Responses []ImageResult `json:"responses"`
	}
	json.Unmarshal(bjson, &results)
	if len(results.Responses) > 0 {
		//		fmt.Println("googlejson:", string(bjson))
		result = results.Responses[0]
	}

	return
}

/*
func SpeechToText() (err error) {

	const usage = `Usage: wordoffset <audiofile>
Audio file must be a 16-bit signed little-endian encoded
with a sample rate of 16000.
The path to the audio file may be a GCS URI (gs://...).
`

	ctx := context.Background()
	client, err := speech.NewClient(ctx)

	gcsURI := ""
	// Send the contents of the audio file with the encoding and
	// and sample rate information to be transcripted.
	req := &speech.LongRunningRecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:              speechpb.RecognitionConfig_LINEAR16,
			SampleRateHertz:       16000,
			LanguageCode:          "en-US",
			EnableWordTimeOffsets: true,
		},
		Audio: &speech.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Uri{Uri: gcsURI},
		},
	}

	op, err := client.AsyncRecognize(ctx, req)
	if err != nil {
		return err
	}
	resp, err := op.Wait(ctx)
	if err != nil {
		return err
	}

	// Print the results.
	for _, result := range resp.Results {
		for _, alt := range result.Alternatives {
			fmt.Printf("\"%v\" (confidence=%3f)\n", alt.Transcript, alt.Confidence)
			for _, w := range alt.Words {
				fmt.Printf("Word: \"%v\" (startTime=%3f, endTime=%3f)\n",
					w.Word,
					float64(w.StartTime.Seconds)+float64(w.StartTime.Nanos)*1e-9,
					float64(w.EndTime.Seconds)+float64(w.EndTime.Nanos)*1e-9,
				)
			}
		}
	}
	return nil
}
*/
