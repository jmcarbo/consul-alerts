package notifier

import (
	"bytes"
	"fmt"
	"log"

	"html/template"
	"net/smtp"
)

type EmailNotifier struct {
	ClusterName string
	Template    string
	Url         string
	Port        int
	Username    string
	Password    string
	SenderAlias string
	SenderEmail string
	Receivers   []string
}

type EmailData struct {
	ClusterName  string
	SystemStatus string
	FailCount    int
	WarnCount    int
	PassCount    int
	Nodes        map[string][]Message
}

func (e EmailData) IsCritical() bool {
	return e.SystemStatus == "CRITICAL"
}

func (e EmailData) IsWarning() bool {
	return e.SystemStatus == "UNSTABLE"
}

func (e EmailData) IsPassing() bool {
	return e.SystemStatus == "OK"
}

func (emailNotifier *EmailNotifier) Notify(alerts []Message) bool {

	overAllStatus, pass, warn, fail := summarize(alerts)
	nodeMap := mapByNodes(alerts)

	e := EmailData{
		ClusterName:  emailNotifier.ClusterName,
		SystemStatus: overAllStatus,
		FailCount:    fail,
		WarnCount:    warn,
		PassCount:    pass,
		Nodes:        nodeMap,
	}

	var tmpl *template.Template
	var err error
	if emailNotifier.Template == "" {
		tmpl, err = template.New("base").Parse(defaultTemplate)
	} else {
		tmpl, err = template.ParseFiles(emailNotifier.Template)
	}

	if err != nil {
		log.Println("Template error, unable to send email notification: ", err)
		return false
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, e); err != nil {
		log.Println("Template error, unable to send email notification: ", err)
		return false
	}

	msg := ""
	msg += fmt.Sprintf("From: \"%s\" <%s>\n", emailNotifier.SenderAlias, emailNotifier.SenderEmail)
	msg += fmt.Sprintf("Subject: %s is %s\n", emailNotifier.ClusterName, overAllStatus)
	msg += "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg += body.String()

	addr := fmt.Sprintf("%s:%d", emailNotifier.Url, emailNotifier.Port)
	auth := smtp.PlainAuth("", emailNotifier.Username, emailNotifier.Password, emailNotifier.Url)
	if err := smtp.SendMail(addr, auth, emailNotifier.SenderEmail, emailNotifier.Receivers, []byte(msg)); err != nil {
		log.Println("Unable to send notification:", err)
		return false
	}
	log.Println("Email notification sent.")
	return true
}

func summarize(alerts []Message) (overallStatus string, passCount, warnCount, failCount int) {
	for _, alert := range alerts {
		switch alert.Status {
		case "passing":
			passCount++
		case "warning":
			warnCount++
		case "critical":
			failCount++
		}
	}
	if failCount != 0 {
		overallStatus = "CRITICAL"
	} else if warnCount != 0 {
		overallStatus = "UNSTABLE"
	} else {
		overallStatus = "OK"
	}
	return
}

func mapByNodes(alerts []Message) map[string][]Message {
	nodeMap := make(map[string][]Message)
	for _, alert := range alerts {
		nodeName := alert.Node
		nodeChecks := nodeMap[nodeName]
		if nodeChecks == nil {
			nodeChecks = make([]Message, 0)
		}
		nodeChecks = append(nodeChecks, alert)
		nodeMap[nodeName] = nodeChecks
	}
	return nodeMap
}

var defaultTemplate string = `
<!DOCTYPE html>
<html lang="en">
	<head>
  		<title>{{ .ClusterName }}</title>
	</head>

	<body style="width:100% !important; min-width: 100%; -webkit-text-size-adjust:100%; -ms-text-size-adjust:100%; margin:0; padding:0; font-family: 'Helvetica', 'Arial', sans-serif; color: #000000;">

		<div style="margin-left: auto; margin-right: auto; width: 36em; padding: 10dp; font-weight: bold; color: #ffffff; background-color: {{ if .IsCritical }}#e13329{{ else if .IsWarning }}#eebb00{{ else if .IsPassing }}#24c75a{{ end }};">
			<div style="padding: 10px;">
				{{ .ClusterName }}
			</div>
		</div>

		<div style="margin-left: auto; margin-right: auto; width: 36em; margin-top: 10px; margin-bottom: 10px; padding: 10dp">
			<p>
			<span style="font-weight: bold; font-size: 1.05em;">System is {{ .SystemStatus }}</span>
			<br/>
			<span style="font-size: 0.9em;">The following nodes are currently experiencing issues:</span>
			<div style="font-size: 0.85em;">
				<div style="float: left; width: 33%;">
					<strong>Failed: </strong>
					<span>{{ .FailCount }}</span>
				</div>
				<div style="float: right; width: 33%;">
					<strong>Warning: </strong>
					<span>{{ .WarnCount }}</span>
				</div>
				<div style="display: inline-block; width: 33%;">
					<strong>Passed: </strong>
					<span>{{ .PassCount }}</span>
				</div>
			</div>
			</p>

		</div>

		{{ range $name, $checks := .Nodes }}
		<div style="margin-left: auto; margin-right: auto; width: 36em; padding-top: 5px; padding-bottom: 20px;">
			<div style="font-size: 1.1em;">
				<strong>Node: </strong>
				<strong>{{ $name }}</strong>
			</div>
			
			{{ range $check := $checks }}
			<div style="margin-top: 15px; padding: 10px; background-color: {{ if $check.IsCritical }}#e13329{{ else if $check.IsWarning }}#eebb00{{ else if $check.IsPassing }}#24c75a{{ end }};">
				<div style="font-weight: bold; font-size: 1.1em;">
					{{ with $check.Service }}
					{{ $check.Service }}:  
					{{ end }}
					{{ $check.Check }}
				</div>
				<div style="font-size: 0.85em;">
					<strong>Since: </strong>
					<span>{{ $check.Timestamp }}</span>
				</div>
				{{ with $check.Notes }}
				<div style="padding-top: 15px;">
					<strong>Notes: </strong>
					<pre>{{ $check.Notes }}</pre>
				</div>
				{{end }}
				<div style="padding-top: 15px;">
					<strong>Output:</strong>
					<pre>{{ $check.Output }}</pre>
				</div>
			</div>
			{{ end }}

		</div>
		{{ end }}

	
	</body>

</html>
`