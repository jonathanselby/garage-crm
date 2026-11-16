package web

import (
 "net/url"
 "strings"
 "testing"
 "time"
 "github.com/jonathanselby/garage-crm/internal/app"
)

func TestCoolingDashboard(t *testing.T) {
 s:=serverTest(t)
 brand,err:=s.Service.Save("brands","",map[string]string{"name":"StagZero"},0)
 if err!=nil {t.Fatal(err)}
 person,err:=s.Service.Save("people","",map[string]string{"name":"<script>cool</script>"},0)
 if err!=nil {t.Fatal(err)}
 if err=s.Service.LinkContactToBrand(brand,person,"Prospect","unknown",false);err!=nil {t.Fatal(err)}
 old:=time.Now().UTC().Add(-16*24*time.Hour).Format(time.RFC3339Nano)
 if err=s.Service.DB.Exec("UPDATE people SET created_at=? WHERE id=?",old,person);err!=nil {t.Fatal(err)}
 w:=req(t,s,"GET","/?brand="+url.QueryEscape(brand),nil,true,"")
 body:=w.Body.String()
 if w.Code!=200 || !strings.Contains(body,"Cooling") || !strings.Contains(body,"16d quiet") || !strings.Contains(body,"/record?type=people&amp;id="+person) {
  t.Fatalf("missing cooling panel: %d %s",w.Code,body)
 }
 if strings.Contains(body,"<script>cool</script>") || !strings.Contains(body,"&lt;script&gt;cool&lt;/script&gt;") {t.Fatalf("unescaped HTML: %s",body)}
 _=app.CoolingDays
}
