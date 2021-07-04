package report

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jharlap/good-day-app/reflection"
	"github.com/jharlap/good-day-app/urlsigner"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type InterruptionsMeetingsReport struct {
	baseURL  string
	signer   *urlsigner.Engine
	db       *sqlx.DB
	renderer *RenderService
}

func New(baseURL string, signer *urlsigner.Engine, db *sqlx.DB, renderURL, renderCredsFile string) *InterruptionsMeetingsReport {

	return &InterruptionsMeetingsReport{
		baseURL:  baseURL,
		signer:   signer,
		db:       db,
		renderer: &RenderService{URL: renderURL, CredentialsFile: renderCredsFile},
	}
}

func (imr *InterruptionsMeetingsReport) URLForTeamAndUser(teamID, userID string, tz int) string {
	sig := imr.signer.Sign(urlsigner.Params{
		TeamID:         teamID,
		UserID:         userID,
		TZ:             tz,
		ExpiryDuration: time.Hour * 24 * 30,
	})
	return fmt.Sprintf("%s/%s", imr.baseURL, sig)
}

func (imr *InterruptionsMeetingsReport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rp urlsigner.Params
	{
		i := strings.LastIndex(r.URL.Path, "/")
		if i < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		p, err := imr.signer.Parse(r.URL.Path[i+1:])
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

	start := mondayOfWeekBeforeInUTC(time.Now(), rp.TZ)
	rows, err := imr.db.QueryxContext(r.Context(), "SELECT * FROM reflections WHERE `date` >= ? AND team_id = ? AND user_id = ?", start.Format(mysqlDatetimeFormat), rp.TeamID, rp.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error().Err(err).Str("tid", rp.TeamID).Str("uid", rp.UserID).Msg("error querying for reflections")
		return
	}
	defer rows.Close()

	var rr []reflection.Reflection
	for rows.Next() {
		var r reflection.Reflection
		err := rows.StructScan(&r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Err(err).Msg("error scanning row")
			return
		}

		// display dates in user timezone
		r.Date = r.Date.Add(-1 * time.Duration(rp.TZ) * time.Hour)

		rr = append(rr, r)
	}
	if err = rows.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error().Err(err).Msg("error getting reflections data")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	err = imr.renderReflectionsEchart(rr, start, start.Add(time.Hour*14*24), w)
	if err != nil {
		log.Error().Err(err).Str("tid", rp.TeamID).Str("uid", rp.UserID).Msg("error rendering report")
	}
}

func mondayOfWeekBeforeInUTC(t time.Time, tzOffset int) time.Time {
	dayOffset := int(t.Weekday()-time.Monday)%7 + 7
	for dayOffset <= 0 {
		dayOffset += 7
	}

	mon := time.Date(t.Year(), t.Month(), t.Day()-dayOffset, 0, 0, 0, 0, time.UTC)
	return mon.Add(-1 * time.Duration(tzOffset) * time.Hour)
}

const mysqlDatetimeFormat = "2006-01-02 15:04:05"
