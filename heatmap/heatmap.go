package heatmap

import (
	"errors"
	"fmt"
	"image/color"
	"net/http"
	"strings"
	"time"

	"github.com/jharlap/good-day-app/reflection"
	"github.com/jharlap/good-day-app/urlsigner"
	"github.com/jmoiron/sqlx"
	"github.com/nikolaydubina/calendarheatmap/charts"
	"github.com/rs/zerolog/log"
	"golang.org/x/image/font"
)

type Heatmap struct {
	baseURL           string
	defaultColorScale charts.BasicColorScale
	defaultFontFace   font.Face
	signer            *urlsigner.Engine
	db                *sqlx.DB
}

func New(baseURL string, signer *urlsigner.Engine, db *sqlx.DB, fontFaceBytes []byte) *Heatmap {
	fontFace, err := charts.LoadFontFace(fontFaceBytes)
	if err != nil {
		log.Fatal().Err(err).Msg("error loading font face")
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
		signer: signer,
		db:     db,
	}
}

func (h *Heatmap) URLForTeamAndUser(teamID, userID string, tz int) string {
	sig := h.signer.Sign(urlsigner.Params{
		TeamID:         teamID,
		UserID:         userID,
		TZ:             tz,
		ExpiryDuration: time.Hour * 24 * 30,
	})
	return fmt.Sprintf("%s/%s", h.baseURL, sig)
}

func (h *Heatmap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rp urlsigner.Params
	{
		i := strings.LastIndex(r.URL.Path, "/")
		if i < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		p, err := h.signer.Parse(r.URL.Path[i+1:])
		if errors.Is(err, urlsigner.ErrInvalidSignature) {
			w.WriteHeader(http.StatusUnauthorized)
			log.Debug().Err(err).Str("path", r.URL.Path).Msg("invalid url signing")
			return
		} else if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Debug().Err(err).Str("path", r.URL.Path).Msg("invalid url signing")
			return
		}
		rp = p
	}

	startOfYear := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC).Format(mysqlDateFormat)
	rows, err := h.db.QueryxContext(r.Context(), "SELECT * FROM reflections WHERE DATE(`date`) >= ? AND team_id = ? AND user_id = ?", startOfYear, rp.TeamID, rp.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error().Err(err).Msgf("error querying for day quality calendar for uid %s", rp.UserID)
		return
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var r reflection.Reflection
		err := rows.StructScan(&r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Err(err).Msg("error scanning row")
			return
		}
		counts[r.Date.Add(-1*time.Duration(rp.TZ)*time.Hour).Format(mysqlDateFormat)] = r.WorkDayQuality.IntVal()
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

	w.Header().Set("Content-Type", "image/png")
	err = charts.WriteHeatmap(conf, w)
	if err != nil {
		log.Error().Err(err).Msg("error writing heatmap image")
	}
}

const mysqlDateFormat = "2006-01-02"
