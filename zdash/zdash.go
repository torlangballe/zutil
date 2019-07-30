package zdash

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/uhttp"
	"github.com/torlangballe/zutil/ztime"
)

// https://bitmovin.com/dynamic-adaptive-streaming-http-mpeg-dash/
// https://www.brendanlong.com/the-structure-of-an-mpeg-dash-mpd.html
// https://bitmovin.com/dynamic-adaptive-streaming-http-mpeg-dash/
// http://mpeg.chiariglione.org/standards/mpeg-dash
// https://www.brendanlong.com/the-structure-of-an-mpeg-dash-mpd.html
// http://standards.iso.org/ittf/PubliclyAvailableStandards/MPEG-DASH_schema_files/DASH-MPD.xsd

// ConditionalUint (ConditionalUintType) defined in XSD as a union of unsignedInt and boolean.
type ConditionalUint struct {
	u *uint64
	b *bool
}

// MarshalXMLAttr encodes ConditionalUint.
func (c ConditionalUint) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	if c.u != nil {
		return xml.Attr{Name: name, Value: strconv.FormatUint(*c.u, 10)}, nil
	}

	if c.b != nil {
		return xml.Attr{Name: name, Value: strconv.FormatBool(*c.b)}, nil
	}

	// both are nil - no attribute, client will threat it like "false"
	return xml.Attr{}, nil
}

// UnmarshalXMLAttr decodes ConditionalUint.
func (c *ConditionalUint) UnmarshalXMLAttr(attr xml.Attr) error {
	u, err := strconv.ParseUint(attr.Value, 10, 64)
	if err == nil {
		c.u = &u
		return nil
	}

	b, err := strconv.ParseBool(attr.Value)
	if err == nil {
		c.b = &b
		return nil
	}

	return fmt.Errorf("ConditionalUint: can't UnmarshalXMLAttr %#v", attr)
}

// check interfaces
var (
	_ xml.MarshalerAttr   = ConditionalUint{}
	_ xml.UnmarshalerAttr = &ConditionalUint{}
)

// MPD represents root XML element.
type MPD struct {
	XMLNS                      *string `xml:"xmlns,attr"`
	Type                       *string `xml:"type,attr"`
	MinimumUpdatePeriod        *string `xml:"minimumUpdatePeriod,attr"`
	AvailabilityStartTime      *string `xml:"availabilityStartTime,attr"`
	MediaPresentationDuration  *string `xml:"mediaPresentationDuration,attr"`
	MinBufferTime              *string `xml:"minBufferTime,attr"`
	SuggestedPresentationDelay *string `xml:"suggestedPresentationDelay,attr"`
	TimeShiftBufferDepth       *string `xml:"timeShiftBufferDepth,attr"`
	PublishTime                *string `xml:"publishTime,attr"`
	Profiles                   string  `xml:"profiles,attr"`
	BaseURL                    string  `xml:"BaseURL,omitempty"`
	Period                     *Period `xml:"Period,omitempty"`

	DurationSecs float64
	RawBytes     []byte
}

type SimpleProfile struct {
	ID                string
	DurationSecs      float64
	Width             int
	Height            int
	Bandwidth         int
	Mime              string
	Lang              string
	Codecs            string
	AudioSamplingRate string
	FrameRate         string
	Chunks            []SimpleChunk
	InitURL           string
}

type SimpleChunk struct {
	StubURL string
	Pos     float64
}

func (m *MPD) GetSimpleProfiles(surl string) (profiles []SimpleProfile, err error) {
	urlPrefix, _ := path.Split(surl)
	if m.BaseURL != "" {
		urlPrefix = m.BaseURL
	}

	if m.Period != nil {
		for _, as := range m.Period.AdaptationSets {
			var sp SimpleProfile
			if as.BitstreamSwitching != nil {
				//				addToList(list, "bitstream switching:", ustr.BoolToStr(*as.BitstreamSwitching))
			}
			sp.Mime = as.MimeType
			if as.Lang != nil {
				sp.Lang = *as.Lang
			}

			for _, r := range as.Representations {
				var durs []float64
				st := r.SegmentTemplate
				if st == nil {
					st = as.SegmentTemplate
				}
				if st == nil {
					continue
				}
				if st.Timescale == nil {
					continue
				}
				media := ""
				if st.Media != nil {
					media = *st.Media
				}
				if st.Duration != nil && st.Timescale != nil {
					sp.DurationSecs = float64(*st.Duration) / float64(*st.Timescale)
				} else if st.Timescale != nil && st.SegmentTimelineS != nil && len(st.SegmentTimelineS) != 0 {
					for _, segtl := range st.SegmentTimelineS {
						repeat := 1
						if segtl.R != nil {
							repeat = int(*segtl.R)
						}
						for i := 0; i < repeat; i++ {
							durs = append(durs, float64(segtl.D)/float64(*st.Timescale))
						}
					}
				}
				p := sp
				p.ID = *r.ID
				if r.Width != nil {
					p.Width = int(*r.Width)
				}
				if r.Height != nil {
					p.Height = int(*r.Height)
				}
				if r.Bandwidth != nil {
					p.Bandwidth = int(*r.Bandwidth)
				}
				if r.Codecs != nil {
					p.Codecs = *r.Codecs
				}
				if r.AudioSamplingRate != nil {
					p.AudioSamplingRate = *r.AudioSamplingRate
				}
				if r.FrameRate != nil {
					p.FrameRate = *r.FrameRate
				}
				media = strings.Replace(media, "$RepresentationID$", *r.ID, 1)
				mInit := strings.Replace(*st.Initialization, "$RepresentationID$", *r.ID, 1)
				p.InitURL = urlPrefix + mInit
				sid := *st.StartNumber

				if len(durs) != 0 && st.StartNumber != nil {
					pos := 0.0
					for i, d := range durs {
						var chunk SimpleChunk
						str := fmt.Sprintf("%d", *st.StartNumber+uint64(i))
						m := strings.Replace(media, "$Number$", str, 1)
						mUrl := urlPrefix + m
						chunk.StubURL = mUrl
						chunk.Pos = pos
						pos += d
						p.Chunks = append(p.Chunks, chunk)
					}
				} else {
					for s := 0.0; s < m.DurationSecs; s += sp.DurationSecs {
						var chunk SimpleChunk
						str := fmt.Sprintf("%d", sid)
						m := strings.Replace(media, "$Number$", str, 1)
						mUrl := urlPrefix + m
						chunk.StubURL = mUrl
						chunk.Pos = s
						sid++
						p.Chunks = append(p.Chunks, chunk)
					}
				}
				profiles = append(profiles, p)
			}
		}
	}
	return
}

// Decode parses MPD XML.
func (m *MPD) Decode(b []byte) error {
	return xml.Unmarshal(b, m)
}

// Period represents XSD's PeriodType.
type Period struct {
	Start          *string          `xml:"start,attr"`
	ID             *string          `xml:"id,attr"`
	Duration       *string          `xml:"duration,attr"`
	AdaptationSets []*AdaptationSet `xml:"AdaptationSet,omitempty"`
}

// AdaptationSet represents XSD's AdaptationSetType.
type AdaptationSet struct {
	MimeType                string           `xml:"mimeType,attr"`
	SegmentAlignment        ConditionalUint  `xml:"segmentAlignment,attr"`
	SubsegmentAlignment     ConditionalUint  `xml:"subsegmentAlignment,attr"`
	StartWithSAP            *uint64          `xml:"startWithSAP,attr"`
	SubsegmentStartsWithSAP *uint64          `xml:"subsegmentStartsWithSAP,attr"`
	BitstreamSwitching      *bool            `xml:"bitstreamSwitching,attr"`
	Lang                    *string          `xml:"lang,attr"`
	ContentProtections      []Descriptor     `xml:"ContentProtection,omitempty"`
	SegmentTemplate         *SegmentTemplate `xml:"SegmentTemplate,omitempty"`
	Representations         []Representation `xml:"Representation,omitempty"`
}

// Representation represents XSD's RepresentationType.
type Representation struct {
	ID                 *string          `xml:"id,attr"`
	Width              *uint64          `xml:"width,attr"`
	Height             *uint64          `xml:"height,attr"`
	FrameRate          *string          `xml:"frameRate,attr"`
	Bandwidth          *uint64          `xml:"bandwidth,attr"`
	AudioSamplingRate  *string          `xml:"audioSamplingRate,attr"`
	Codecs             *string          `xml:"codecs,attr"`
	ContentProtections []Descriptor     `xml:"ContentProtection,omitempty"`
	SegmentTemplate    *SegmentTemplate `xml:"SegmentTemplate,omitempty"`
}

// Descriptor represents XSD's DescriptorType.
type Descriptor struct {
	SchemeIDURI *string `xml:"schemeIdUri,attr"`
	Value       *string `xml:"value,attr"`
}

// SegmentTemplate represents XSD's SegmentTemplateType.
type SegmentTemplate struct {
	Timescale              *uint64            `xml:"timescale,attr"`
	Media                  *string            `xml:"media,attr"`
	Initialization         *string            `xml:"initialization,attr"`
	StartNumber            *uint64            `xml:"startNumber,attr"`
	PresentationTimeOffset *uint64            `xml:"presentationTimeOffset,attr"`
	SegmentTimelineS       []SegmentTimelineS `xml:"SegmentTimeline>S,omitempty"`
	Duration               *uint64            `xml:"duration,attr"`
}

// SegmentTimelineS represents XSD's SegmentTimelineType's inner S elements.
type SegmentTimelineS struct {
	T *uint64 `xml:"t,attr"`
	D uint64  `xml:"d,attr"`
	R *int64  `xml:"r,attr"`
}

func GetFromUrl(surl string) (m *MPD, err error) {
	resp, err := http.Get(surl)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	err = uhttp.CheckErrorFromBody(resp)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	m = new(MPD)
	err = m.Decode(body)
	if err == nil {
		if m.MediaPresentationDuration != nil {
			d, derr := ztime.DurationStructFromIso(*m.MediaPresentationDuration)
			if derr != nil {
				err = derr
				fmt.Println("zdash.GetFromUrl err getting duration:", err)
				return
			}
			m.DurationSecs = ztime.DurSeconds(d.ToDuration())
		}
	}
	m.RawBytes = body

	return
}
