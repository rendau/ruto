"""Test stand: configures a fresh core for the SSO checks (see CLAUDE.md,
"Тестовый стенд"): first admin, provider corp (Keycloak in docker), two apps with
one GET endpoint each, snapshot published. Run with python3 -I once core is up."""
import json, urllib.request, sys
BASE='http://localhost:9090/api'
TOKEN=None
def call(method, path, body=None):
    data=json.dumps(body).encode() if body is not None else None
    req=urllib.request.Request(BASE+path, data=data, method=method, headers={'Content-Type':'application/json', **({'Authorization':'Bearer '+TOKEN} if TOKEN else {})})
    try:
        with urllib.request.urlopen(req) as r: return r.status, json.loads(r.read() or b'{}')
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()

st,rep=call('GET','/usr/bootstrap/status'); print('bootstrap', st, rep)
if rep.get('can_create_first_admin'):
    print('create admin', call('POST','/usr',{'name':'Test Admin','username':'admin','password':'ruto-test-admin','is_admin':True,'active':True,'all_apps':True}))
st,rep=call('POST','/usr/login',{'username':'admin','password':'ruto-test-admin'}); print('login', st); TOKEN=rep['jwt']

st,root=call('GET','/root'); print('root get', st)
root.update({
  'base_url':'http://localhost:8088',
  'cors':{'enabled':True,'allow_credentials':True,'max_age':'1h','allow_origins':['http://localhost:5181','http://localhost:5182'],'allow_methods':['*'],'allow_headers':['*']},
  'jwt':[{'name':'corp','issuer':'http://localhost:8180/realms/corp','public_url':'http://localhost:8088','client':{'id':'ruto','secret':'ruto-secret','scopes':[]}}],
})
root.pop('apps',None)
print('root post', call('POST','/root',root))

def app(name,prefix,auth_roles,jwt=None):
    body={'active':True,'name':name,'path_prefix':prefix,'backend':{'url':'http://127.0.0.1:18081'},
          'auth':{'enabled':True,'mode':'replace','methods':[{'jwt':{'provider':'corp','roles':auth_roles}}]}}
    if jwt: body['jwt']=jwt
    st,rep=call('POST','/app',body); print('app',name,st,rep); aid=rep['id']
    st,rep=call('POST','/endpoint',{'app_id':aid,'active':True,'type':'http','http':{'method':'GET','path':'hello'},'auth':{'enabled':True,'mode':'extend','methods':[]}}); print('endpoint',name,st,rep)
    return aid

app('shop','shop',['auditor'])
app('orders','orders',['shop:manager'],jwt=[{'provider':'corp','client':{'id':'orders-client','secret':'orders-secret','scopes':[]}}])
print('deploy', call('POST','/snapshot/deploy',{}))
print('version', call('GET','/snapshot/version'))
