package main

import (
	"gorm.io/gorm"
	"strings"
)

func migrateTasksV1toV2(db *gorm.DB) {

	splitTitle := func(t string) (jiraId string, title string) {
		jiraId = ""
		title = t
		if strings.HasPrefix(t, "[") {
			brPos := strings.Index(t, "]")
			if brPos != -1 {
				jiraId = t[0 : brPos+1]
				title = t[brPos+2:]
			}
		}
		return
	}

	var tasks []Task
	result := db.Where("title like '[%' or title like '%]%'").Find(&tasks)
	if result.RowsAffected > 0 {
		for _, t := range tasks {
			jira_id, newTitle := splitTitle(t.Title)
			if jira_id != "" {
				t.JiraId = jira_id
				t.Title = newTitle
				db.Save(&t)
			}
		}
	}

}
