{{template "base" .}}

{{define "title"}}{{.Title}}{{end}}

{{define "content"}}
<h4>{{.Title}}</h4>

<h6>Last Updated: {{.FormattedLastUpdated}}</h6>

<table class="striped">
    <thead>
        <tr>
            <th>Name</th>
            <th>Price</th>
            <th>Savings</th>
        </tr>
    </thead>

    <tbody>
        {{range .Products}}
        <tr>
            <td><a href="{{.URL}}" target="_blank">{{.Name}}</a></td>
            <td>{{.Price}}</td>
            <td>{{.Savings}}</td>
        </tr>
        {{else}}
        <tr>
            <td>No Deals :(</td>
        </tr>
        {{end}}
    </tbody>
</table>
{{end}}