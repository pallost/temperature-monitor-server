package main

import (
	"html/template"
	"net/http"
	"encoding/json"
	"time"

	"appengine"
	"appengine/datastore"
	"net/url"
	"log"
	"io/ioutil"
)

type Measurement struct {
	Temperature float32
	Humidity    float32
	Date        int64
}

func init() {
	http.HandleFunc("/", showMeasurements)
	http.HandleFunc("/get", getMeasurements)
	http.HandleFunc("/latest", latestMeasurements)
	http.HandleFunc("/add", addMeasurement)
}

func measurementKey(c appengine.Context) *datastore.Key {
	return datastore.NewKey(c, "Measurement", "default_measurement", 0, nil)
}

func getCurrentTemperature() float32 {
	forecaUrl := "https://www.foreca.com/lv"
	tampereId := "100634963"

	response, err := http.PostForm(forecaUrl, url.Values{
		"id": {tampereId},
	})

	if err != nil {
		log.Println(err)
		return -1
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		log.Println(err)
		return -1
	}

	log.Println(string(body))

	return 1
}

func latestMeasurements(resp http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)

	q := datastore.NewQuery("Measurement").Ancestor(measurementKey(ctx)).Order("-Date").Limit(1)

	measurements := make([]Measurement, 0, 1)
	if _, err := q.GetAll(ctx, &measurements); err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusCreated)
	json.NewEncoder(resp).Encode(measurements)

	getCurrentTemperature()
}

func getMeasurements(resp http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)

	var filter = time.Now().Add(-7 * 24 * time.Hour).UnixNano() / int64(time.Millisecond)
	var limit = 500
	// Ancestor queries, as shown here, are strongly consistent with the High
	// Replication Datastore. Queries that span entity groups are eventually
	// consistent. If we omitted the .Ancestor from this query there would be
	// a slight chance that Greeting that had just been written would not
	// show up in a query.
	// Filter("Date >", filter).
	q := datastore.NewQuery("Measurement").Ancestor(measurementKey(ctx)).Filter("Date >", filter).Order("-Date").Limit(limit)

	measurements := make([]Measurement, 0, limit)
	if _, err := q.GetAll(ctx, &measurements); err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusCreated)
	json.NewEncoder(resp).Encode(measurements)

	getCurrentTemperature()
}

func showMeasurements(resp http.ResponseWriter, req *http.Request) {
	err := chartTemplate.Execute(resp, nil)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
}

var chartTemplate = template.Must(template.New("book").Parse(`
<html>
<head>
    <link href="https://cdnjs.cloudflare.com/ajax/libs/nvd3/1.8.6/nv.d3.css" rel="stylesheet">
    <style>
        .nv-y1 .tick line { display: none; }
    </style>
</head>
<body>

<div id="chart"><svg></svg></div>
<script src="https://cdnjs.cloudflare.com/ajax/libs/lodash.js/4.17.4/lodash.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/d3/3.5.17/d3.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/nvd3/1.8.6/nv.d3.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/moment.js/2.22.2/moment.min.js"></script>

<script>
	// refresh every 3 hours
	window.setInterval(function() {
 		window.location.reload();
	}, 3 * 60 * 60 * 1000); 

    var chart = null;

    function renderChart(chartData) {
	  nv.addGraph(function() {
        chart = nv.models.multiChart()
          .margin({top: 30, right: 60, bottom: 50, left: 70})
          .color(d3.scale.category10().range())
          .useInteractiveGuideline(true)
          .showLegend(true);

        chart.xAxis
          .axisLabel('Time (ms)')
          .tickFormat(function(d) {
            return d3.time.format('%m-%d %H:%M')(new Date(d))
          })
          .showMaxMin(false);

        chart.yAxis1
          .axisLabel('C')
          .tickFormat(d3.format('.02f'));

        chart.yAxis2
          .axisLabel('%')
          .tickFormat(d3.format('.02f'));

        d3.select('#chart svg')
          .datum(chartData)
          .call(chart);
  
        nv.utils.windowResize(function() { chart.update() });
        return chart;
      });
    }

    d3.json("/get", function(err, data) {
        data = _.sortBy(data, "Date");
        data = _.filter(data, function (meas) {
          return true;
          // return meas.Temperature > 0 &&
          //       meas.Humidity > 0 &&
			//	 meas.Temperature < 50 &&
		//		 meas.Humidity < 100 &&
		//		 meas.Humidity > meas.Temperature; 
                 // moment(meas.Date).isAfter(moment().subtract(14, 'days'));
        });

        var renderData = [
          {
            values: _.map(data, function(m) {return {x: m.Date, y: m.Temperature}}),
            yAxis: 1,
            type: "line",
            key: 'Temperature',
            color: '#ff7f0e'
          },
          {
            values: _.map(data, function(m) {return {x: m.Date, y: m.Humidity}}),
            yAxis: 2,
            type: "line",
            key: 'Humidity',
            color: '#2ca02c'
          }
        ];

        renderChart(renderData);
	});
</script>
</body>
</html>
`))

func addMeasurement(resp http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)

	var meas Measurement
	err := json.NewDecoder(req.Body).Decode(&meas)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	// We set the same parent key on every Greeting entity to ensure each Greeting
	// is in the same entity group. Queries across the single entity group
	// will be consistent. However, the write rate to a single entity group
	// should be limited to ~1/second.
	key := datastore.NewIncompleteKey(ctx, "Measurement", measurementKey(ctx))

	_, err = datastore.Put(ctx, key, &meas)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(resp, req, "/", http.StatusFound)
}
