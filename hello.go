package guestbook

import (
    "html/template"
    "net/http"
    "encoding/json"

    "appengine"
    "appengine/datastore"
)

type Measurement struct {
    Temperature float32
    Humidity    float32
    Date        int64
}

func init() {
    http.HandleFunc("/", showMeasurements)
    http.HandleFunc("/add", addMeasurement)
}

func measurementKey(c appengine.Context) *datastore.Key {
    return datastore.NewKey(c, "Measurement", "default_measurement", 0, nil)
}

func showMeasurements(resp http.ResponseWriter, req *http.Request) {
    ctx := appengine.NewContext(req)

    var limit = 500
    // Ancestor queries, as shown here, are strongly consistent with the High
    // Replication Datastore. Queries that span entity groups are eventually
    // consistent. If we omitted the .Ancestor from this query there would be
    // a slight chance that Greeting that had just been written would not
    // show up in a query.
    q := datastore.NewQuery("Measurement").Ancestor(measurementKey(ctx)).Order("-Date").Limit(limit)

    measurements := make([]Measurement, 0, limit)
    if _, err := q.GetAll(ctx, &measurements); err != nil {
        http.Error(resp, err.Error(), http.StatusInternalServerError)
        return
    }

    measJson, err := json.Marshal(measurements)
    if err != nil {
        http.Error(resp, err.Error(), http.StatusInternalServerError)
        return
    }


    err = chartTemplate.Execute(resp, string(measJson))
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

<script>
    /*These lines are all chart setup.  Pick and choose which chart features you want to utilize. */
    nv.addGraph(function() {
        var chart = nv.models.lineChart()
            .margin({left: 100})  //Adjust chart margins to give the x-axis some breathing room.
            .useInteractiveGuideline(true)  //We want nice looking tooltips and a guideline!
            .showLegend(true)       //Show the legend, allowing users to turn on/off line series.
            .showYAxis(true)        //Show the y-axis
            .showXAxis(true)        //Show the x-axis
        ;

        chart.xAxis
            .axisLabel('Time (ms)')
            .tickFormat(function(d) {
                return d3.time.format('%m-%d %H:%M')(new Date(d))
            })
            .showMaxMin(false);

        chart.yAxis     //Chart y-axis settings
            .axisLabel('C / %')
            .tickFormat(d3.format('.02f'));

        /* Done setting the chart up? Time to render it!*/
        var myData = getData();   //You need data...

        d3.select('#chart svg')    //Select the <svg> element you want to render the chart in.
            .datum(myData)         //Populate the <svg> element with chart data...
            .call(chart);          //Finally, render the chart!

        //Update the chart when window resizes.
        nv.utils.windowResize(function() { chart.update() });
        return chart;
    });
    /**************************************
     * Simple test data generator
     */
    function getData() {
        var data = JSON.parse({{.}})
        data = _.sortBy(data, "Date")

        //Line chart data should be sent as an array of series objects.
        return [
            {
                values: _.map(data, function(m) {return {x: m.Date, y: m.Temperature}}),
                key: 'Temperature', //key  - the name of the series.
                color: '#ff7f0e'  //color - optional: choose your own line color.
            },
            {
                values: _.map(data, function(m) {return {x: m.Date, y: m.Humidity}}),
                key: 'Humidity',
                color: '#2ca02c'
            }
        ];
    }
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
