README
------

NOTE: This is alpha software

Executes a NRQL query and returns the result in CSV form. The Insights API
doesn't provide a good mechanism for preserving column order, so the returned
columns are not likely to be in the order specified in the query. You will need
NEW_RELIC_ACCOUNT_ID and NEW_RELIC_QUERY_KEY environment variables (for
information about how to get your query key, [see here][0]).

## USAGE

`nrql2csv <nrql-query>`

## EXAMPLE

``` bash
$ nrql2csv "SELECT name, duration FROM transaction LIMIT 8"
duration,timestamp,name
0.001,1491944202715,WebTransaction/Expressjs/GET//s_health
0.001,1491944202453,WebTransaction/Expressjs/GET//s_health
0.001,1491944202327,WebTransaction/Expressjs/GET//s_health
0.001,1491944201970,WebTransaction/Expressjs/GET//s_health
0.007,1491944201382,WebTransaction/Expressjs/GET//health
0.005,1491944190477,WebTransaction/Expressjs/GET//health
0.002,1491944187714,WebTransaction/Expressjs/GET//s_health
0.001,1491944187453,WebTransaction/Expressjs/GET//s_health
```

## INSTALL

### FROM SOURCE

1. `brew install golang` or `apt-get install golang`
2. `go build`
3. Move `./nrql2csv` to a directory in your `$PATH`
4. Add your NewRelic account ID and query key to your `.bash_profile`,
   `.bashrc`, etc:
   * `echo "export NEW_RELIC_ACCOUNT_ID=<account_id>" >> ~/.bash_profile`
   * `echo "export NEW_RELIC_QUERY_KEY=<query_key>" >> ~/.bash_profile`

[0]: https://docs.newrelic.com/docs/insights/export-insights-data/export-api/query-insights-event-data-api#register
