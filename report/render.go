package report

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/jharlap/good-day-app/reflection"
)

func (imr *InterruptionsMeetingsReport) renderReflectionsEchart(rr []reflection.Reflection, startTime, endTime time.Time, w io.Writer) error {
	c := map[string]interface{}{
		"title": map[string]string{
			"text":    "Meetings and interruptions",
			"subtext": "Shaded days are good days",
		},
		"legend": map[string]string{
			"type": "plain",
			"top":  "bottom",
			"left": "center",
		},
		"xAxis": map[string]string{
			"type": "time",
		},
		"yAxis": []map[string]interface{}{
			{
				"type": "category",
				"data": categoryDataForOptionSet(reflection.NumberOptions),
				"axisLine": map[string]interface{}{
					"lineStyle": map[string]string{
						"color": "#c1232b",
						"type":  "dotted",
					},
				},
			},

			{
				"type": "category",
				"data": categoryDataForOptionSet(reflection.AmountOfDayOptions),
				"axisLine": map[string]interface{}{
					"lineStyle": map[string]string{
						"color": "#27727b",
						"type":  "dashed",
					},
				},
			},
		},
		"dataset": map[string]interface{}{
			"dimensions": []map[string]string{
				{"name": "date", "type": "time"},
				{"name": "interruptions", "type": "ordinal"},
				{"name": "meetings", "type": "ordinal"},
			},
			"source": sourceDataForReflections(startTime, endTime, rr),
		},
		"series": []map[string]interface{}{
			{
				"name": "Meetings",
				"type": "line",
				"encode": map[string]string{
					"x": "date",
					"y": "meetings",
				},
				"symbol":     "emptySquare",
				"symbolSize": 10,
				"lineStyle": map[string]string{
					"type": "dotted",
				},
			},
			{
				"name": "Interruptions",
				"type": "line",
				"encode": map[string]string{
					"x": "date",
					"y": "interruptions",
				},
				"yAxisIndex": 1,
				"symbol":     "emptyCircle",
				"symbolSize": 10,
				"lineStyle": map[string]string{
					"type": "dashed",
				},

				"markArea": map[string]interface{}{
					"data": markAreaDataForReflections(rr),
				},
			},
		},
		"color": []string{
			"#c1232b",
			"#27727b",
			"#fcce10",
			"#e87c25",
			"#b5c334",
			"#fe8463",
			"#9bca63",
			"#fad860",
			"#f3a43b",
			"#60c0dd",
			"#d7504b",
			"#c6e579",
			"#f4e001",
			"#f0805a",
			"#26c0c0",
		},
	}

	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("error rendering to json: %w", err)
	}

	img, err := imr.renderer.Render(b)
	if err != nil {
		return fmt.Errorf("error rendering chart: %w", err)
	}

	_, err = w.Write(img)
	return err
}

func categoryDataForOptionSet(os reflection.OptionSet) []string {
	var ss []string
	for _, o := range os.Options {
		ss = append(ss, o.Text)
	}
	return ss
}

func sourceDataForReflections(startTime, endTime time.Time, rr []reflection.Reflection) []map[string]string {
	d := []map[string]string{
		{"date": startTime.Format(dateFormat)},
	}
	for _, r := range rr {
		m := map[string]string{
			"date":          r.Date.Format(dateFormat),
			"meetings":      reflection.NumberOptions.ValueFor(r.ValueForQuestion("meeting_number")),
			"interruptions": reflection.AmountOfDayOptions.ValueFor(r.ValueForQuestion("interrupted_amount")),
		}
		d = append(d, m)
	}
	d = append(d, map[string]string{
		"date": endTime.Format(dateFormat),
	})
	return d
}

func markAreaDataForReflections(rr []reflection.Reflection) [][]map[string]string {
	var d [][]map[string]string
	for _, r := range rr {
		if r.WorkDayQuality.IntVal() >= 3 {
			markStart := time.Date(r.Date.Year(), r.Date.Month(), r.Date.Day()-1, 12, 0, 0, 0, time.UTC)
			markEnd := time.Date(r.Date.Year(), r.Date.Month(), r.Date.Day(), 12, 0, 0, 0, time.UTC)
			d = append(d, []map[string]string{
				{"xAxis": markStart.Format(markAreaDateFormat)},
				{"xAxis": markEnd.Format(markAreaDateFormat)},
			})
		}
	}
	return d
}

const (
	dateFormat         = "2006-01-02"
	markAreaDateFormat = "2006-01-02 15:04"
)
