package heatmap

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nikolaydubina/calendarheatmap/charts"
	"github.com/rs/zerolog/log"
	"golang.org/x/image/font"
)

type Heatmap struct {
	baseURL           string
	defaultColorScale charts.BasicColorScale
	defaultFontFace   font.Face
	hmacKey           []byte
	db                *sqlx.DB
}

func New(baseURL string, hmacKey string, db *sqlx.DB, fontFaceBytes []byte) *Heatmap {
	fontFace, err := charts.LoadFontFace(fontFaceBytes)
	if err != nil {
		log.Fatal().Err(err).Msg("error loading font face")
	}

	hm, err := base64.StdEncoding.DecodeString(hmacKey)
	if err != nil {
		log.Fatal().Err(err).Msg("error decoding url signing key")
	}

	return &Heatmap{
		baseURL:         baseURL,
		defaultFontFace: fontFace,
		defaultColorScale: charts.BasicColorScale{
			color.RGBA{0xEE, 0xEE, 0xEE, 255},
			color.RGBA{0xFF, 0x9F, 0x1C, 255},
			color.RGBA{0xFF, 0xBF, 0x69, 255},
			color.RGBA{0xFF, 0xFF, 0xFF, 255},
			color.RGBA{0xCB, 0xF3, 0xF0, 255},
			color.RGBA{0x2E, 0xC4, 0xB6, 255},
		},
		hmacKey: hm,
		db:      db,
	}
}

func (h *Heatmap) URLForTeamAndUser(teamID, userID string, tz int) (string, error) {
	p := params{
		TeamID: teamID,
		UserID: userID,
		TZ:     tz,
		Expiry: time.Now().Add(time.Hour * 24 * 30).UnixNano(),
	}

	u, err := h.urlForParams(p)
	if err != nil {
		return "", fmt.Errorf("error generating signed URL: %w", err)
	}
	return u, nil
}

func (h *Heatmap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rp params
	if len(r.URL.Path) > len("/heatmap/") {
		p, err := h.paramsFromString(r.URL.Path[len("/heatmap/"):])
		if errors.Is(err, errInvalidHMAC) {
			w.WriteHeader(http.StatusUnauthorized)
			log.Debug().Err(err).Msgf("invalid url signing: %s", r.URL.Path)
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Debug().Err(err).Msgf("invalid url signing: %s", r.URL.Path)
			return
		}
		rp = p
	}

	tzOffset := fmt.Sprintf("%+02d:00", rp.TZ)
	today := time.Now().Add(time.Hour * 24).Format(mysqlDateFormat)
	startOfYear := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC).Format(mysqlDateFormat)
	rows, err := h.db.QueryContext(r.Context(), "SELECT DATE(CONVERT_TZ(`date`, '+00:00', ?)) dt, LEFT(work_day_quality, 1) FROM reflections WHERE DATE(`date`) >= ? AND DATE(`date`) <= ? AND team_id = ? AND user_id = ?", tzOffset, startOfYear, today, rp.TeamID, rp.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error().Err(err).Msgf("error querying for day quality calendar for uid %s", rp.UserID)
		return
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var (
			d string
			v int
		)
		err := rows.Scan(&d, &v)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Err(err).Msg("error scanning row")
			return
		}
		counts[d] = v
	}
	if err = rows.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error().Err(err).Msg("error getting calendar data")
		return
	}

	conf := charts.HeatmapConfig{
		Counts:             counts,
		ColorScale:         h.defaultColorScale,
		DrawMonthSeparator: true,
		DrawLabels:         true,
		Margin:             30,
		BoxSize:            100,
		TextWidthLeft:      350,
		TextHeightTop:      200,
		TextColor:          color.RGBA{0, 0, 0, 255},
		BorderColor:        color.RGBA{200, 200, 200, 255},
		Locale:             "en_US",
		Format:             "png",
		FontFace:           h.defaultFontFace,
		ShowWeekdays: map[time.Weekday]bool{
			time.Monday:    true,
			time.Wednesday: true,
			time.Friday:    true,
		},
	}

	err = charts.WriteHeatmap(conf, w)
	if err != nil {
		log.Error().Err(err).Msg("error writing heatmap image")
	}
}

type params struct {
	TeamID string `json:"t"`
	UserID string `json:"u"`
	TZ     int    `json:"z"`
	Expiry int64  `json:"ts"`
	HMAC   []byte `json:"h"`
}

func (p params) hmac(key []byte) []byte {
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(fmt.Sprintf("%s:%s:%d:%d", p.TeamID, p.UserID, p.TZ, p.Expiry)))
	return mac.Sum(nil)
}

func (h *Heatmap) urlForParams(p params) (string, error) {
	p.HMAC = p.hmac(h.hmacKey)

	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("error encoding heatmap url: %w", err)
	}

	enc := hex.EncodeToString(b)
	return fmt.Sprintf("%s/heatmap/%s", h.baseURL, enc), nil
}

func (h *Heatmap) paramsFromString(s string) (params, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return params{}, fmt.Errorf("error decoding string: %w", err)
	}

	var p params
	err = json.Unmarshal(b, &p)
	if err != nil {
		return params{}, fmt.Errorf("error unmarshaling json: %w", err)
	}

	if p.Expiry < time.Now().UnixNano() {
		return params{}, errExpiredHMAC
	}

	hm := p.hmac(h.hmacKey)
	if !hmac.Equal(hm, p.HMAC) {
		return params{}, errInvalidHMAC
	}

	return p, nil
}

var (
	errInvalidHMAC = errors.New("invalid signature")
	errExpiredHMAC = errors.New("expired signature")
)

const mysqlDateFormat = "2006-01-02"
