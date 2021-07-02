package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "embed"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jharlap/good-day-app/heatmap"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var (
	sapi          *slack.Client
	signingSecret string
	db            *sqlx.DB
	heatmapper    *heatmap.Heatmap
)

//go:embed assets/fonts/Sunflower-Medium.ttf
var defaultFontFaceBytes []byte

func main() {
	urlSigningKeyB64 := os.Getenv("URL_SIGNING_KEY_BASE64")
	baseURL := os.Getenv("BASE_URL")
	signingSecret = os.Getenv("SLACK_SIGNING_SECRET")
	sapi = slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	port := os.Getenv("PORT")
	if len(port) > 0 {
		port = fmt.Sprintf(":%s", port)
	} else {
		port = ":3000"
	}

	{
		dsn := os.Getenv("DATABASE_DSN")
		fmt.Println("connecting to db with dsn:", dsn)
		c, err := sqlx.Open("mysql", dsn)
		if err != nil {
			log.Fatal().Err(err).Msg("error connecting to database")
		}
		db = c
		db.SetConnMaxLifetime(time.Minute * 3)
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(10)
	}

	heatmapper = heatmap.New(baseURL, urlSigningKeyB64, db, defaultFontFaceBytes)
	http.HandleFunc("/", printBody)
	http.Handle("/event", verifySecret(http.HandlerFunc(handleEvent)))
	http.Handle("/interactive", verifySecret(http.HandlerFunc(handleInteractive)))
	http.Handle("/slash", verifySecret(http.HandlerFunc(handleSlash)))
	http.Handle("/heatmap/", heatmapper) // no verifySecret because this is a signed URL
	http.ListenAndServe(port, nil)
}

func verifySecret(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		sv, err := slack.NewSecretsVerifier(r.Header, signingSecret)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, err := sv.Write(body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := sv.Ensure(); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		next.ServeHTTP(w, r)
	})
}

func handleSlash(w http.ResponseWriter, r *http.Request) {
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch s.Command {
	case "/reflect":
		params := &slack.Msg{Text: "Yay! Reflection time!"}
		b, err := json.Marshal(params)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)

		go startReflectionDialog(s.TriggerID)
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func handleInteractive(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Debug().Err(err).Msg("error parsing interactive form")
		return
	}

	body := []byte(r.PostFormValue("payload"))
	var ic slack.InteractionCallback
	err = json.Unmarshal(body, &ic)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Debug().Err(err).Str("body", string(body)).Msg("error unmarshaling interactive body")
		return
	}

	if ic.Type == slack.InteractionTypeBlockActions && len(ic.ActionCallback.BlockActions) > 0 && ic.ActionCallback.BlockActions[0].ActionID == homeButtonStartReflection {
		startReflectionDialog(ic.TriggerID)
	} else if ic.Type == slack.InteractionTypeBlockActions && len(ic.ActionCallback.BlockActions) > 0 && ic.ActionCallback.BlockActions[0].ActionID == homeButtonDownloadData {
		sendDataDownload(ic.Team.ID, ic.User.ID)
	} else if ic.Type == slack.InteractionTypeViewSubmission && ic.View.CallbackID == reflectionModalCallbackID {
		handleReflectionModalCallback(ic)
	}
}

func handleReflectionModalCallback(ic slack.InteractionCallback) {
	/*
		for k, v := range ic.View.State.Values {
			for ik := range v {
				val := ic.View.State.Values[k][ik].SelectedOption.Value
				fmt.Println("k:", k, "ik:", ik, "iv.Value:", val)
			}
		}
	*/
	r := Reflection{
		TeamID:                ic.Team.ID,
		UserID:                ic.User.ID,
		Date:                  time.Now().Format("2006-01-02 15:04:05"),
		WorkDayQuality:        selectedOptionValue(ic, "work_day_quality"),
		WorkOtherPeopleAmount: selectedOptionValue(ic, "work_other_people_amount"),
		HelpOtherPeopleAmount: selectedOptionValue(ic, "help_other_people_amount"),
		InterruptedAmount:     selectedOptionValue(ic, "interrupted_amount"),
		ProgressGoalsAmount:   selectedOptionValue(ic, "progress_goals_amount"),
		QualityWorkAmount:     selectedOptionValue(ic, "quality_work_amount"),
		LotOfWorkAmount:       selectedOptionValue(ic, "lot_of_work_amount"),
		WorkDayFeeling:        selectedOptionValue(ic, "work_day_feeling"),
		StressfulAmount:       selectedOptionValue(ic, "stressful_amount"),
		BreaksAmount:          selectedOptionValue(ic, "breaks_amount"),
		MeetingNumber:         selectedOptionValue(ic, "meeting_number"),
		MostProductiveTime:    selectedOptionValue(ic, "most_productive_time"),
		LeastProductiveTime:   selectedOptionValue(ic, "least_productive_time"),
	}
	err := saveReflection(r)
	if err != nil {
		reportErrorToUser(err, ic.Team.ID, ic.User.ID, fmt.Sprintf("Sorry, I hit a snag and couldn't save your reflection. To make it easier to save, here's your answers: %s", r))
		log.Error().Err(err).Msg("error saving reflection")
		return
	}

	messageUser(ic.Team.ID, ic.User.ID, fmt.Sprintf("Well done! I saved your reflection - here is what you said:\n%s", r))
}

func selectedOptionValue(ic slack.InteractionCallback, field string) NumberPrefixedEnum {
	return NumberPrefixedEnum(ic.View.State.Values[field]["select"].SelectedOption.Value)
}

func saveReflection(r Reflection) error {
	_, err := db.Exec("INSERT INTO reflections SET team_id=?, user_id=?, date=?, work_day_quality=?, work_other_people_amount=?, help_other_people_amount=?, interrupted_amount=?, progress_goals_amount=?, quality_work_amount=?, lot_of_work_amount=?, work_day_feeling=?, stressful_amount=?, breaks_amount=?, meeting_number=?, most_productive_time=?, least_productive_time=?", r.TeamID, r.UserID, r.Date, r.WorkDayQuality, r.WorkOtherPeopleAmount, r.HelpOtherPeopleAmount, r.InterruptedAmount, r.ProgressGoalsAmount, r.QualityWorkAmount, r.LotOfWorkAmount, r.WorkDayFeeling, r.StressfulAmount, r.BreaksAmount, r.MeetingNumber, r.MostProductiveTime, r.LeastProductiveTime)
	if err != nil {
		return fmt.Errorf("error saving reflection: %w", err)
	}

	return nil
}

func generateReflectionModal() slack.ModalViewRequest {
	// Create a ModalViewRequest with a header and two inputs
	titleText := slack.NewTextBlockObject(slack.PlainTextType, "Good Day Tracker", false, false)
	closeText := slack.NewTextBlockObject(slack.PlainTextType, "Close", false, false)
	submitText := slack.NewTextBlockObject(slack.PlainTextType, "Submit", false, false)

	headerText := slack.NewTextBlockObject(slack.MarkdownType, "Time to think about how the day went. Pick the answers that are closest to how you felt today went, and we'll review for patterns at the end of the week.", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	bb := []slack.Block{headerSection}
	for _, q := range Questions {
		bb = append(bb, q.SlackBlock())
	}

	blocks := slack.Blocks{
		BlockSet: bb,
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = slack.ViewType("modal")
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	modalRequest.CallbackID = reflectionModalCallbackID
	return modalRequest
}

func startReflectionDialog(triggerID string) error {
	v := generateReflectionModal()
	_, err := sapi.OpenView(triggerID, v)
	if err != nil {
		return fmt.Errorf("error opening reflection modal: %w", err)
	}
	return nil
}

func handleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ev, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Debug().Err(err).Str("body", string(body)).Msg("unable to parse event")
	}

	switch ev.Type {
	case slackevents.URLVerification:
		if uv, ok := ev.Data.(*slackevents.EventsAPIURLVerificationEvent); ok {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(uv.Challenge))
		} else {
			log.Debug().Str("body", string(body)).Msg("wrong type cast for URLVerification")
			fmt.Printf("data: %+v\n", ev.Data)
		}

	case slackevents.CallbackEvent:
		handleInnerEvent(r.Context(), w, ev.InnerEvent)

	default:
		fmt.Printf("ev: %+v\n", ev)
		log.Debug().Err(err).Str("evtype", ev.Type).Msg("unknown event type")

	}
}

func handleInnerEvent(ctx context.Context, w http.ResponseWriter, iev slackevents.EventsAPIInnerEvent) {
	switch ev := iev.Data.(type) {
	case *slackevents.AppHomeOpenedEvent:
		bb, err := renderHomeView(ctx, ev.View.TeamID, ev.User)
		if err != nil {
			log.Debug().Err(err).Str("user", ev.User).Msg("error rendering home view")
			w.WriteHeader(http.StatusInternalServerError)
		}

		v := slack.HomeTabViewRequest{
			Type:   slack.VTHomeTab,
			Blocks: bb,
		}
		r, err := sapi.PublishViewContext(ctx, ev.User, v, "")
		if err != nil {
			log.Debug().Err(err).Str("user", ev.User).Msgf("error publishing home view: %+v", r.ResponseMetadata.Messages)
		}

	default:
		fmt.Printf("unknown inner event type: %+v", ev)
	}
}

func renderHomeView(ctx context.Context, tid, uid string) (slack.Blocks, error) {
	var bb slack.Blocks

	u, err := sapi.GetUserInfoContext(ctx, uid)
	if err != nil {
		return bb, fmt.Errorf("error getting user info for uid %s: %w", uid, err)
	}

	bb.BlockSet = append(bb.BlockSet, slack.NewSectionBlock(nil, []*slack.TextBlockObject{slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("hello *%s*!", u.Name), false, false)}, nil))

	hmURL, err := heatmapper.URLForTeamAndUser(tid, uid, u.TZOffset/3600)
	if err != nil {
		return bb, fmt.Errorf("error getting heatmap URL for tid %s uid %s: %w", tid, uid, err)
	}

	bb.BlockSet = append(bb.BlockSet, slack.NewImageBlock(hmURL, "Daily feeling at a glance", "", slack.NewTextBlockObject(slack.PlainTextType, "Daily feeling at a glance", false, false)))

	bb.BlockSet = append(bb.BlockSet, slack.NewActionBlock(
		"home-start-reflection-action-block",
		slack.NewButtonBlockElement(homeButtonStartReflection, "start-today-btn", slack.NewTextBlockObject(slack.PlainTextType, "Reflect on Today", false, false)),
		slack.NewButtonBlockElement(homeButtonDownloadData, "download-data-btn", slack.NewTextBlockObject(slack.PlainTextType, "Download Reflections Data", false, false)),
	))

	return bb, nil
}

func userReflectionsCSV(tid, uid string) (string, error) {
	rows, err := db.Queryx("SELECT * FROM reflections WHERE team_id = ? AND user_id = ?", tid, uid)
	if err != nil {
		return "", err
	}

	isFirstRow := true
	buf := new(bytes.Buffer)
	cw := csv.NewWriter(buf)
	for rows.Next() {
		if isFirstRow {
			nn, err := rows.Columns()
			if err != nil {
				return "", fmt.Errorf("error reading column metadata: %w", err)
			}
			cw.Write(nn)
			isFirstRow = false
		}

		r, err := rows.SliceScan()
		if err != nil {
			return "", fmt.Errorf("error scanning db row: %w", err)
		}

		ss, err := stringSlice(r)
		if err != nil {
			return "", fmt.Errorf("error converting db row: %w", err)
		}

		err = cw.Write(ss)
		if err != nil {
			return "", fmt.Errorf("error writing csv row: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error reading csv rows: %w", err)
	}

	// if no rows, insert data to indicate an empty file
	if isFirstRow {
		return "No reflections found.", nil
	}

	cw.Flush()
	return buf.String(), nil
}

func sendDataDownload(tid, uid string) {
	log.Info().Str("tid", tid).Str("uid", uid).Msg("sendDataDownload")
	content, err := userReflectionsCSV(tid, uid)
	if err != nil {
		reportErrorToUser(err, tid, uid, "Sorry, there was an error uploading your data to Slack - please try again in a few minutes.")
		return
	}

	fn := fmt.Sprintf("reflections_%s_%d.csv", uid, time.Now().Unix())
	_, err = sapi.UploadFile(slack.FileUploadParameters{
		Title:    fn,
		Filename: fn,
		Filetype: "csv",
		Content:  content,
		Channels: []string{uid},
	})
	if err != nil {
		reportErrorToUser(err, tid, uid, "Sorry, there was an error uploading your data to Slack - please try again in a few minutes.")
		return
	}

	_, _, err = sapi.PostMessage(
		uid,
		slack.MsgOptionText("All your reflections to date are in this file. Please note that date columns are in UTC timezone.", false),
	)
	if err != nil {
		log.Error().Err(err).Msgf("error posting file explanation error message to %s", uid)
	}
}

func stringSlice(ii []interface{}) ([]string, error) {
	var ss []string
	for _, i := range ii {
		switch v := i.(type) {
		case string:
			ss = append(ss, v)
		case uint8:
			ss = append(ss, strconv.FormatUint(uint64(v), 10))
		case []uint8:
			ss = append(ss, string([]byte(v)))
		default:
			return nil, fmt.Errorf("error converting row value of type %T to string: %+v\n", v, v)
		}
	}
	return ss, nil
}

func messageUser(tid, uid, msg string) {
	_, _, err := sapi.PostMessage(
		uid,
		slack.MsgOptionText(msg, false),
	)
	if err != nil {
		log.Error().Err(err).Msgf("error posting error report to %s", uid)
	}
}

func reportErrorToUser(err error, tid, uid, msg string) {
	log.Error().Err(err).Str("tid", tid).Str("uid", uid).Msg("")
	messageUser(tid, uid, msg)
}

func printBody(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	fmt.Println(string(b), err)
}

const (
	homeButtonStartReflection = "start-reflection-action"
	homeButtonDownloadData    = "download-data-action"
	reflectionModalCallbackID = "reflection-modal-callback-id"
)
