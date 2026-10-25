var cur=null;
document.addEventListener('DOMContentLoaded',function(){fetchData();setInterval(fetchData,2000);});
function fetchData(){fetch('?api=dashboard_data').then(function(r){return r.json();}).then(function(d){if(!d.ok)return;cur=d;renderUI(d);}).catch(function(e){console.error(e);});}
function switchTab(e,t){document.querySelectorAll('.tab-pane').forEach(function(x){x.classList.remove('active');});document.querySelectorAll('.nav-item').forEach(function(x){x.classList.remove('active');});document.getElementById('pane-'+t).classList.add('active');e.currentTarget.classList.add('active');if(innerWidth<=768)toggleSidebar();if(t==='logs')loadLog(document.getElementById('log-selector').value);}
function toggleSidebar(){document.getElementById('sidebar').classList.toggle('open');document.getElementById('overlay').classList.toggle('active');}
function toggleCompactMode(c){document.getElementById('tableContainer').classList.toggle('compact-mode',c);}
function loadLog(t){var el=document.getElementById('log-display');el.innerText='Loading...';fetch('?api=get_log&type='+t+'&lines=100').then(function(r){return r.json();}).then(function(d){if(d.ok){el.innerText=d.content;el.scrollTop=el.scrollHeight;}else{el.innerText='Error';}}).catch(function(e){el.innerText='Gagal: '+e.message;});}
function setMode(m){fetch('?api=set_mode&mode='+m).then(fetchData).catch(function(e){alert('Error: '+e);});}
function setSyncMode(m){fetch('?api=set_sync_mode&mode='+m).then(fetchData).catch(function(e){alert('Error: '+e);});}
function triggerScan(){fetch('?api=scan').then(function(){setTimeout(fetchData,1000);});}
function syncNow(){fetch('?api=sync_now').then(function(r){return r.json();}).then(function(d){if(!d.ok)alert(d.error||'Gagal');setTimeout(fetchData,1500);});}
function pauseSync(p){fetch('?api=set_backup&state='+(p?'pause':'resume')).then(fetchData);}
function retrySync(){fetch('?api=retry_sync').then(function(r){return r.json();}).then(function(d){alert('Reset '+d.reset+' file gagal ke antrian.');setTimeout(fetchData,1000);});}
function num(v,d){return (v==null||v==='')?'-':Number(v).toFixed(d==null?2:d);}
var DEFS='<defs><marker id="ah" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#94a3b8"/></marker><marker id="ahb" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#3b82f6"/></marker><marker id="ahr" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#ef4444"/></marker></defs>';
function rect(cx,cy,w,h,label,sub,cls){var x=cx-w/2,y=cy-h/2;return '<g class="nd '+cls+'"><rect x="'+x+'" y="'+y+'" width="'+w+'" height="'+h+'"/><text x="'+cx+'" y="'+(sub?cy-7:cy)+'">'+label+'</text>'+(sub?'<text class="sub" x="'+cx+'" y="'+(cy+9)+'">'+sub+'</text>':'')+'</g>';}
function dia(cx,cy,w,h,label,cls){var hw=w/2,hh=h/2;return '<g class="nd dia '+cls+'"><polygon points="'+cx+','+(cy-hh)+' '+(cx+hw)+','+cy+' '+cx+','+(cy+hh)+' '+(cx-hw)+','+cy+'"/><text x="'+cx+'" y="'+cy+'">'+label+'</text></g>';}
function edge(x1,y1,x2,y2,cls){return '<g class="ed '+cls+'"><path d="M'+x1+','+y1+' L'+x2+','+y2+'"/></g>';}
function elbow(x1,y1,x2,y2,cls){var mx=(x1+x2)/2;return '<g class="ed '+cls+'"><path d="M'+x1+','+y1+' H'+mx+' V'+y2+' H'+x2+'"/></g>';}
function bypass(x1,y1,x2,y2,topY,label,cls){return '<g class="bp '+cls+'"><path d="M'+x1+','+y1+' V'+topY+' H'+x2+' V'+y2+'"/><text x="'+((x1+x2)/2)+'" y="'+(topY-6)+'">'+label+'</text></g>';}
function buildLive(on,force){var cy=150,A=70,B=210,C=360,D=510,E=660,Fx=830,Fy=90,Gx=830,Gy=210,s=DEFS;
s+=edge(A+55,cy,B-60,cy,on?'on flow':'');s+=edge(B+60,cy,C-60,cy,on?'on flow':'');s+=edge(C+60,cy,D-60,cy,on?'on flow':'');s+=edge(D+60,cy,E-60,cy,on?'on flow':'');
s+=elbow(E+60,cy,Fx-50,Fy,on?'on':'');s+=elbow(E+60,cy,Gx-50,Gy,on?'on':'');
s+=bypass(B,cy-40,D,cy-44,40,'FORCE BYPASS (lewat Jadwal)',force?'on':'');
s+=rect(A,cy,100,44,'Start Loop','',on?'on glow':'');s+=dia(B,cy,120,80,'Cek Sistem',on?'on glow':'');s+=dia(C,cy,120,80,'Cek Waktu',on?'on glow':'');
s+=rect(D,cy,110,48,'Kompresi','X265 / WebP',on?'on glow':'');s+=dia(E,cy,120,80,'Bandingkan',on?'on glow':'');
s+=rect(Fx,Fy,100,40,'Replace','','rep'+(on?' on glow':''));s+=rect(Gx,Gy,110,40,'Skip (Boros)','','skp'+(on?' on glow':''));
return s;}
function buildSync(on){var cy=60,A=80,B=280,C=480,s=DEFS;
s+=edge(A+55,cy,B-55,cy,on?'on flow':'');s+=edge(B+55,cy,C-55,cy,on?'on flow':'');
s+=rect(A,cy,110,44,'File Lokal','/mnt/hdd1/...',on?'on glow':'');s+=rect(B,cy,110,44,'Rclone','Encrypt (opt)',on?'on glow':'');
s+=rect(C,cy,120,44,'MEGA Cloud','/Backup/.../','rep'+(on?' on glow':''));return s;}
function renderUI(d){var sys=d.system,st=d.stats,files=d.recent_files,syncF=d.backup_queue,ap=d.active_processes;
var on=!!(ap.compression&&ap.compression.file),bon=!!(ap.backup&&ap.backup.file);
var b=document.getElementById('top-mode-badge');b.className='mode-badge mode-'+sys.effective_mode;b.innerText=sys.effective_mode;
var p=document.getElementById('top-pos-status');var pk=(sys.position_status||'PENDING').split(' ')[0].replace('(SCHEDULE)','').replace('(FORCE)','').trim();p.className='pos-status pos-'+pk;p.innerText=sys.position_status;
var sm=(sys.sync_force?'FORCE':'NORMAL');var sb=document.getElementById('top-sync-badge');
if(sys.backup_pause){sb.className='sync-badge s-pause';sb.innerText='⏸ SYNC '+sm;}
else if(bon){sb.className='sync-badge '+(sys.sync_force?'s-force':'s-up');sb.innerText='⬆ SYNC '+sm;}
else{sb.className='sync-badge '+(sys.sync_force?'s-force':'s-normal');sb.innerText='SYNC '+sm;}
var tt=document.getElementById('top-temp');tt.innerText=sys.temp_c+'°C';tt.className=(sys.temp_c>=78?'t-hot':(sys.temp_c>=65?'t-warn':'t-ok'));
document.getElementById('top-ram').innerText=sys.ram_used_mb+'/'+sys.ram_total_mb+' MB';
document.getElementById('hs-saved-mb').innerText=st.saved_mb+' MB';
document.getElementById('hs-saved-comp').innerText='Dari Asli: '+st.total_orig_mb+' MB';
document.getElementById('hs-success').innerText=st.success_count;
document.getElementById('hs-skipped').innerText=st.skipped_boros_count;
document.getElementById('hs-error').innerText=st.error_count;
document.getElementById('hs-processing').innerText=st.processing_count;
document.getElementById('sub-v').innerText=st.processing_video;
document.getElementById('sub-p').innerText=st.processing_photo;
document.getElementById('hs-pending').innerText=st.pending_count;
['resume','pause','normal','force'].forEach(function(id){document.getElementById('btn-'+id).classList.remove('active-pause','active-force','active-normal');});
if(sys.manual_pause)document.getElementById('btn-pause').classList.add('active-pause');
else if(sys.force_mode)document.getElementById('btn-force').classList.add('active-force');
else document.getElementById('btn-normal').classList.add('active-normal');
var tb=document.getElementById('tbl-body');tb.innerHTML='';
files.forEach(function(f){var tr=document.createElement('tr');var nm=f.path.split('/').pop();if(nm.length>25)nm=nm.substring(0,22)+'...';
var s=f.status;if(f.status==='DONE_COMPRESSED')s='<span class="c-green">Sukses</span>';else if(f.status==='SKIPPED_BOROS')s='<span class="c-yellow">Skip</span>';else if(f.status==='ERROR')s='<span class="c-red">Gagal</span>';else if(f.status==='PROCESSING')s='<span class="c-blue">Proses</span>';else if(f.status==='PENDING')s='Antri';
var ct=(f.category&&f.category!=='UNKNOWN'&&f.category!=='')?'<span class="cat-tag cat-'+f.category+'">'+f.category+'</span>':'-';
var hp=(f.percent_saved==null)?'-':Number(f.percent_saved).toFixed(1)+'%';
tr.innerHTML='<td title="'+f.path+'">'+nm+'</td><td>'+(f.media_type==='video'?'V':'P')+'</td><td>'+s+'</td><td class="hide-compact">'+num(f.original_size_mb,2)+'</td><td>'+num(f.final_size_mb,2)+'</td><td>'+hp+'</td><td>'+ct+'</td><td class="hide-compact" style="color:var(--mut);font-size:.75rem">'+f.updated_at+'</td>';tb.appendChild(tr);});
var ss=0,sz=0,sq=0,se=0;syncF.forEach(function(f){if(f.backup_status==='SUCCESS'){ss++;sz+=(f.final_size_mb||0);}else if(f.backup_status==='UPLOADING'||!f.backup_status)sq++;else if(f.backup_status&&(f.backup_status.indexOf('ERROR')>=0||f.backup_status.indexOf('FAIL')>=0))se++;});
document.getElementById('sync-total-files').innerText=ss;document.getElementById('sync-total-size').innerText=(sz/1024).toFixed(2)+' GB';
document.getElementById('sync-queue').innerText=sq;document.getElementById('sync-errors').innerText=se;
var pb=document.getElementById('btn-sync-pause'),rb=document.getElementById('btn-sync-resume');
pb.classList.toggle('active-pause',!!sys.backup_pause);rb.classList.toggle('active-normal',!sys.backup_pause);
var sn=document.getElementById('btn-sync-normal'),sf2=document.getElementById('btn-sync-force');
if(sn&&sf2){sn.classList.toggle('active-normal',!sys.sync_force);sf2.classList.toggle('active-force',!!sys.sync_force);}
var sby=document.getElementById('tbl-sync-body');sby.innerHTML='';
syncF.forEach(function(f){var tr=document.createElement('tr');var nm=f.path.split('/').pop();if(nm.length>25)nm=nm.substring(0,22)+'...';
var bs=f.backup_status||'PENDING';if(bs==='SUCCESS')bs='<span class="c-green">Uploaded</span>';else if(bs==='UPLOADING')bs='<span class="c-blue">Uploading</span>';else if(bs.indexOf('ERROR')>=0||bs.indexOf('FAIL')>=0)bs='<span class="c-red">Fail</span>';else bs='<span class="c-yellow">Queued</span>';
tr.innerHTML='<td>'+nm+'</td><td>'+(f.media_type==='video'?'V':'P')+'</td><td>'+num(f.final_size_mb,2)+' MB</td><td>'+bs+'</td><td style="color:var(--mut);font-size:.75rem">'+f.updated_at+'</td>';sby.appendChild(tr);});
document.getElementById('live-svg').innerHTML=buildLive(on,sys.force_mode);
document.getElementById('sync-svg').innerHTML=buildSync(bon&&!sys.backup_pause);
var dd=document.getElementById('process-details-content');var h='';
if(on){var pc=ap.compression.percent||0,sg=ap.compression.stage||'processing';
h+='<div class="process-stage"><div class="stage-title">Kompresi Aktif</div><div class="stage-name">'+ap.compression.file+'</div><div class="stage-sub">Tahap: '+sg+'</div><div class="progress-track"><div class="progress-fill" style="width:'+pc+'%"></div></div><div class="progress-text">'+pc+'%</div></div>';}
if(bon){var pc2=ap.backup.percent||0,sp=ap.backup.speed||'...';
h+='<div class="process-stage" style="border-left-color:var(--green)"><div class="stage-title">Backup Aktif</div><div class="stage-name">'+ap.backup.file+'</div><div class="stage-sub">Speed: '+sp+'</div><div class="progress-track"><div class="progress-fill green" style="width:'+pc2+'%"></div></div><div class="progress-text">'+pc2+'%</div></div>';}
dd.innerHTML=h||'<div class="empty-note">Tidak ada proses aktif saat ini.</div>';}
