{{define "base"}}
<!DOCTYPE html>
<html>
    <head>
        <title>{{block "title" .}}Home{{end}} - Ebucks Dealz</title>
        <link href="https://fonts.googleapis.com/icon?family=Material+Icons" rel="stylesheet">
        <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/css/materialize.min.css">

        <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    </head>

    <body>

    <nav>
        <div class="nav-wrapper container">
            <a href="{{.PathPrefix}}" class="brand-logo">Ebucks Dealz</a>
        </div>
    </nav>

    <div class="container">

    {{template "content" .}}

    </div>

    <script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/js/materialize.min.js"></script>
    </body>
</html>
{{end}}