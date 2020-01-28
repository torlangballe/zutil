// https://www.sitepoint.com/html5-file-drag-and-drop/

firstLoad = true;

const zReturnKey = 13;
const zUpKey = 38;
const zDownKey = 40;

var zBigTime = new Date("2200-01-01T00:00:00+00:00");
var aceEditor = null;

function isEmpty(obj) {
  if (!obj) {
    return true;
  }
  return Object.keys(obj).length === 0 && obj.constructor === Object;
}

function zIsElementDisplayed(e) {
  if (e.style.display == 'none') {
    return false;
  }
  return true;
}

function zDisplayElement(e, show) {
  if (show) {
    e.style.display = 'block';
  } else {
    e.style.display = 'none';
  }
}

function zMakeVisibleElementById(id, show) {
  var e = document.getElementById(id);
  if (show) {
    e.style.visibility = 'visible';
  } else {
    e.style.visibility = 'hidden';
  }
}

function zDisplayElementById(id, show) {
  var e = document.getElementById(id);
  zDisplayElement(e, show); // let it fail on null so we get error/log
}

function zEnableElementById(id, enable) {
  var e = document.getElementById(id);
  e.disable = !enable;
}

function randomN(n) {
  if (n) {
    return Math.floor(Math.random() * n);
  }
  return Math.floor(Math.random() * 4294967294);
}

function getParameterByName(name) {
  var match = RegExp('[?&]' + name + '=([^&]*)').exec(window.location.search);
  return match && decodeURIComponent(match[1].replace(/\+/g, ' '));
}

function isEmpty(obj) {
  if (!obj) {
    return true;
  }
  return Object.keys(obj).length === 0 && obj.constructor === Object;
}

function zKeyValues(obj, got) {
  for (var key in obj) {
    if (obj.hasOwnProperty(key)) {
      got(key, obj[key]);
    }
  }
}

function createCORSRequest(method, url) {
  var xhr = new XMLHttpRequest();
  if ("withCredentials" in xhr) {
//    console.log("createCORSRequest:", method, url);
    // XHR for Chrome/Firefox/Opera/Safari.
    xhr.open(method, url, true);
  } else if (typeof XDomainRequest != "undefined") {
    // XDomainRequest for IE.
    xhr = new XDomainRequest();
    xhr.open(method, url);
  } else {
    // CORS not supported.
    xhr = null;
  }
  return xhr;
}

function getZoneOffsetString() {
    var zoff = -(new Date()).getTimezoneOffset() / 60;
    return zoff;
}

function getHeadersFromString(str) {
    var headers = {};
    var all = str.split("\r\n");
    for (i = 0; i < all.length; i++) {
        var parts = all[i].split(": ");
        if (parts.length == 2) {
            headers[parts[0]] = parts[1];
        }
    }
    return headers;
}
// info can be method (GET default), contentType, headers, body, notJSONResult=false for raw, 
function makeCorsRequest(url, info, got, err, progress) {
  //  console.log("do cors: " + url);
  var method = 'GET';
  if (info.method) {
    method = info.method;
  }
  var xhr = createCORSRequest(method, url);
  if (!xhr) {
    console.log('CORS not supported');
    return;
  }
  //  console.log("createCORSRequest:", method, url);

  if (info.contentType) {
    xhr.setRequestHeader('Content-Type', info.contentType);
  }
  if (info.headers) {
    zKeyValues(info.headers, function(k, v) {
      xhr.setRequestHeader(k, v);
    });
  }
  xhr.setRequestHeader('X-TimeZone-Offset-Hours', getZoneOffsetString());
  xhr.onload = function() {
    if (progress) {
      progress(1);
    }
    var text = xhr.responseText;
//    console.log("got cors:", xhr.status, text, info.notJSONResult);
    var obj = text;
    if (info.notJSONResult !== true) {
      obj = zParseJSON(text);
      if (obj == false) {
        var str = "makeCorsRequest: error parsing json: " + url + ": " + text;
        console.log(str);
        if (err) {
          err(0, str);
        }
        return;
      }
    }
    if (xhr.status >= 300) {
      var m = zParseJSON(text);
      if (m !== false && m.messages && obj.messages.length > 0 && err) {
          err(xhr.status, obj.messages[0]);
      } else if (err) {
          err(xhr.status, "error");
      }
      return;
    }
    var headers = getHeadersFromString(xhr.getAllResponseHeaders());
    if (got) {
      got(obj, headers);
    }
  };

  xhr.onabort = function() {
    var str = 'abort in cors request: ' + url
    console.log(str);
    if (err) {
      err(299, str);
    }
  }

  xhr.onerror = function() {
      console.log('Woops, there was an error making the cors request: ' + xhr.status + ' ' + url);
    if (err) {
      err(xhr.status, "");
    }
  };

  if (progress) {
    xhr.upload.onprogress = function(event) {
      if (event.lengthComputable) {
        var complete = (event.loaded / event.total | 0);
        progress(complete);
      }
    };
  }
  if (!info.body && !info.formData && !info.file) {
    xhr.send();
  } else {   
    if (info.formData) {
      xhr.send(info.formData);
      return;
    }
    if (info.file) {
      xhr.setRequestHeader("Content-Type", 'application/octet-stream');
      console.log("read file:", info.file);
      var reader = new FileReader();
      reader.onload = function(evt) {
        xhr.send(evt.target.result);
      }
      reader.readAsArrayBuffer(info.file);
    } else if (info.body) {
      xhr.send(info.body);
    }
  }
}

function zDrawProgressCircleInId(id, t, color) {
  var c = document.getElementById(id);
  var ctx = c.getContext("2d");
  var r = Math.min(c.width, c.height) * 0.4;

  zClearCanvas(ctx, c);
  ctx.beginPath();
  ctx.lineWidth = 4;
  ctx.lineCap = "round";

  ctx.arc(c.width / 2 + 1, c.height / 2, r, 0, t * 2 * Math.PI);
  ctx.strokeStyle = color;
  ctx.stroke();
}

function zClearCanvas(context, canvas) {
  context.clearRect(0, 0, canvas.width, canvas.height);
}

function zClearCanvasById(id) {
  var c = document.getElementById(id);
  var ctx = c.getContext("2d");
  zClearCanvas(ctx, c);
}

function createCookie(name, value, days) {
  var expires;
  if (days) {
    var date = new Date();
    date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
    expires = "; expires=" + date.toGMTString();
  } else {
    expires = "";
  }
  document.cookie = name + "=" + value + expires + "; path=/";
}

function getCookie(c_name) {
  if (document.cookie.length > 0) {
    c_start = document.cookie.indexOf(c_name + "=");
    if (c_start != -1) {
      c_start = c_start + c_name.length + 1;
      c_end = document.cookie.indexOf(";", c_start);
      if (c_end == -1) {
        c_end = document.cookie.length;
      }
      return unescape(document.cookie.substring(c_start, c_end));
    }
  }
  return "";
}


function generalEnableField(id, icon) {
  var e = document.getElementById(id);
  if (e.disabled === true) {
    e.disabled = false;
    icon.src = "/assets/images/lock.png"
  } else {
    e.disabled = true;
    icon.src = "/assets/images/unlock.png"
  }
}

function popupUrl(url, w, h, title) {
  if (!w) {
    w = 400;
  }
  if (!h) {
    h = 200;
  }
  if (!title) {
    title = '';
  }
  var x = screen.width / 2 - w / 2;
  var y = screen.height / 2 - h / 2;
  window.open(url, title, 'height=' + h + ',width=' + w + ',left=' + x + ',top=' + y);
  return false;
}

function openWindowInNewTab(url) {
  var win = window.open(url, "_blank");
  win.focus();

  return win;
}

/*

var getUserMedia = navigator.getUserMedia || navigator.webkitGetUserMedia || navigator.mozGetUserMedia;

class Camera {
  constructor(video, canvas, height = 320, width = 320) {
    this.isStreaming = false; // maintain the state of streaming
    this.height = height;
    this.width = width;

    // need a canvas and a video in order to make this work.
    this.canvas = canvas || $(document.createElement('canvas'));
    this.video = video || $(document.createElement('video'));
  }


  getStream() {
    console.log("getusermedia:");
    console.log(navigator.getUserMedia);
    console.log(navigator.webkitGetUserMedia);
    console.log(navigator.mozGetUserMedia);
    return new Promise(function(resolve, reject) {
      getUserMedia({
        video: true
      }, resolve, reject);
    });
  }

  streamToX(stream, element) {
    var video = this.video;
    return new Promise(function(resolve, reject) {
      element.appendChild(video);
      video.src = window.URL.createObjectURL(stream);
      video.onloadedmetadata = function(e) {
        video.play();
        resolve(video);
      }
    });
  }

  streamTo(stream, video) {
    return new Promise(function(resolve, reject) {
      video.src = window.URL.createObjectURL(stream);
      video.onloadedmetadata = function(e) {
        video.play();
        resolve(video);
      }
    });
  }

  takePicture() {
    var image = new Image();
    var canv = this.canvas.get(0)
    var context = canv.getContext('2d');
    context.drawImage(this.video.get(0), 0, 0, this.width, this.height);
    var data = canv.toDataUrl('image/png');
    image.src = data;
    return image;
  }
}

function zLoadScriptAsync(url, callback) {
  var script = document.createElement("script")
  script.type = "text/javascript";
  script.onload = function() {
    callback();
  };
  script.src = url;
  document.getElementsByTagName("head")[0].appendChild(script);
}
*/

function zLoadAceAsyncAndSetup(elementId) {
  zLoadScriptAsync("https://cdnjs.cloudflare.com/ajax/libs/ace/1.2.6/ace.js", function() {
    zLoadScriptAsync("https://cdnjs.cloudflare.com/ajax/libs/ace/1.2.6/ext-language_tools.js", function() {
      zSetupAceEditor(elementId);
    })
  })
}


function zSetupAceEditor(name) {
  var langTools = ace.require("ace/ext/language_tools");
  aceEditor = ace.edit(name);

  aceEditor.setTheme("ace/theme/ambiance");
  aceEditor.session.setMode("ace/mode/json");
  aceEditor.renderer.setShowGutter(false);
  aceEditor.renderer.setOption('showLineNumbers', false);
  aceEditor.setShowPrintMargin(false);
  aceEditor.setOptions({
    enableBasicAutocompletion: true, // the editor completes the statement when you hit Ctrl + Space
    enableLiveAutocompletion: true, // the editor completes the statement while you are typing          
  });
  aceEditor.$blockScrolling = Infinity;
  aceEditor.commands.addCommand({
    name: 'save',
    bindKey: {
      win: 'Ctrl-S',
      mac: 'Command-S'
    },
    exec: function(editor) {
      saveEditorSessionString(editor);
    },
    readOnly: true // false if this command should not apply in readOnly mode
  });
  var staticWordCompleter = {
    getCompletions: function(editor, session, pos, prefix, callback) {
      if (prefix.length === 0) {
        callback(null, []);
        return
      }
      var wordList = ["VOICE", "JINGLE", "BACKGROUND", "ONEOF"];
      callback(null, wordList.map(function(word) {
        return {
          caption: word,
          value: word,
          meta: "static"
        };
      }));
    }
  }
  aceEditor.completers.push(staticWordCompleter);
}

var editorStateStr = "";

function saveEditorSessionString(editor) {
  var session = editor.session
  var state = {}
    //    state.value = session.getValue();
  state.selection = session.selection.toJSON()
  state.folds = session.getAllFolds().map(function(fold) {
    return {
      start: fold.start,
      end: fold.end,
      placeholder: fold.placeholder
    };
  });
  state.scrollTop = session.getScrollTop()
  state.scrollLeft = session.getScrollLeft()
  stateStr = JSON.stringify(state);
  console.log(stateStr);
  console.log("saveState:", stateStr);
}

var Range = null;

function setEditorSessionFromString(editor) {
  if (Range == null) {
    Range = ace.require('ace/range').Range;
  }
  console.log("setState:", stateStr);
  var state = JSON.parse(stateStr);
  editor.session.selection.fromJSON(state.selection)
  try {
    state.folds.forEach(function(fold) {
      console.log(fold.placeholder);
      console.log(fold.start);
      console.log(fold.end);
      var range = Range.fromPoints(fold.start, fold.end);
      editor.session.addFold(fold.placeholder, range);
      //                new Range.fromPoints(fold.start, fold.end));
    });
  } catch (e) {
    console.log(e);
  }
  editor.session.setScrollTop(state.scrollTop)
  editor.session.setScrollTop(state.scrollLeft)
}

function saveEditor(editor) {
  setEditorSessionFromString(editor);
}

function getMonthName3(month) { // getMonth() is zero-based
  switch (month) {
    case 0:
      return "Jan";
    case 1:
      return "Feb";
    case 2:
      return "Mar";
    case 3:
      return "Apr";
    case 4:
      return "May";
    case 5:
      return "Jun";
    case 6:
      return "Jul";
    case 7:
      return "Aug";
    case 8:
      return "Sep";
    case 9:
      return "Oct";
    case 10:
      return "Nov";
    case 11:
      return "Dec";
  }
}

Date.prototype.getFullString = function() {
  var yyyy = this.getUTCFullYear();
  var mon = getMonthName3(this.getUTCMonth()); // getMonth() is zero-based
  var d = this.getUTCDate();
  var dd = d < 10 ? "0" + d : d;
  var h = this.getUTCHours();
  var hh = h < 10 ? "0" + h : h;
  var m = this.getUTCMinutes();
  var min = m < 10 ? "0" + m : m;
  //   var ss = this.getSeconds() < 10 ? "0" + this.getSeconds() : this.getSeconds();
  return dd + "-" + mon + "-" + yyyy + " " + hh + ":" + min;
};

function makeRandom24BitNumber() {
  return Math.floor(Math.random() * 16777215);
}

function makeRandomHex() {
  return makeRandom24BitNumber().toString(16) + makeRandom24BitNumber().toString(16) + makeRandom24BitNumber().toString(16);
}

function setParameterToWindowUrl(key, value) {
  var url = window.location.href;
  if (url.indexOf('?') > -1) {
    url += "&"
  } else {
    url += "?"
  }
  url += key + "=" + value;
  window.location.href = url;
}

function zParseJSON(jsonString) {
  try {
    var o = JSON.parse(jsonString);

    // Handle non-exception-throwing cases:
    // Neither JSON.parse(false) or JSON.parse(1234) throw errors, hence the type-checking,
    // but... JSON.parse(null) returns null, and typeof null === "object", 
    // so we must check for that, too. Thankfully, null is falsey, so this suffices:
    if (o && typeof o === "object") {
      return o;
    }
  } catch (e) {}

  return false;
}

var zRegNumeric = new RegExp('^[0-9.]+');

function zIsStringNumericWithDot(str) {
  var r = zRegNumeric.exec(str);
  if (r == null) {
    return false;
  }
  if (r[0] == str) {
    return true;
  }
  return false;
}

function zGetCurrentGeoPos() {
  if (navigator.geolocation) {
    navigator.geolocation.getCurrentPosition(function(pos) {
      return {
        x: pos.coords.longitude,
        y: pos.coords.latitude
      };
    });
  }
  return null;
}

function zSetMapCenter(map, x, y) {
  //  console.log("zSetMapCenter:", x, y);
  var loc = new google.maps.LatLng(y, x);
  map.setCenter(loc);
}

function zGetSplitAndTrimmedString(str, sep, isnum) {
  if (str == null || str == "") {
    return [];
  }
  var parts = str.split(sep);
  for (i = 0; i < parts.length; i++) {
    parts[i] = parts[i].trim();
    if (isnum) {
      parts[i] = parseFloat(parts[i]);
    }
  }
  return parts;
}

function zGetObjectAsParameters(dict, sep) {
    console.log("zGetObjectAsParameters:", sep);
    var str = "";
    for (var key in dict) {
        if (str != "") {
            str += sep;
        }
        str += key + "=" + dict[key];
    }
    return str;
}

function zParametersToObject(str, sep) {
    console.log("zParametersToObject", str);
    var o = {};
    var a = str.split(sep);
    for (i = 0; i < a.length; i++) {
        var sa = a[i].split("=");
        if (sa.length == 2) {
            o[sa[0]] = sa[1]; 
        }
    }
    return o;
}

function zArrayRemovedItem(array, item) {
  for (i = 0; i < array.length; i++) {
    if (array[i] == item) {
      array.splice(i, 1);
      break;
    }
  }
  return array;
}

function zArrayAddItemUnique(array, item) {
  for (i = 0; i < array.length; i++) {
    if (array[i] == item) {
      return array;
    }
  }
  array.push(item);
  return array;
}


function zGetIPInfo(got) {
  // {"as":"AS2116 Broadnet AS","city":"Oslo","country":"Norway","countryCode":"NO",
  // "isp":"Broadnet AS","lat":59.905,"lon":10.7487,"org":"Broadnet AS","query":"193.90.175.217",
  // "region":"03","regionName":"Oslo County","status":"success","timezone":"Europe/Oslo",
  // "zip":"0001"}
  makeCorsRequest("http://ip-api.com/json", {}, function(obj, headers) {
    console.log("gotIP:", obj);
    got(obj);
  });
}

zMarkers = {};

function zAddCircleMarker(map, x, y, radius, id, color) {
  //  console.log("zAddCircleMarker", x, y);
  if (!radius) {
    radius = 0;
  }
  if (!color) {
    color = "#00F";
  }
  var o = {
    strokeColor: color,
    strokeOpacity: 0.8,
    strokeWeight: 1,
    fillColor: '#003',
    fillOpacity: 0.35,
    center: {
      lat: y,
      lng: x
    },
    map: map,
    radius: radius,
    zIndex: 100000,
    draggable: true,
    id: id
  };
  var circle = new google.maps.Circle(o);
  zMarkers[id] = circle;

  return circle;
}

function zGetGMapStaticCircleParamter(lat, lng, rad, detail) {
  var uri = 'https://maps.googleapis.com/maps/api/staticmap?';
  var staticMapSrc = 'center=' + lat + ',' + lng;
  staticMapSrc += '&size=100x100';
  staticMapSrc += '&path=color:0xff0000ff:weight:1';

  var r = 6371;

  var pi = Math.PI;

  var _lat = (lat * pi) / 180;
  var _lng = (lng * pi) / 180;
  var d = (rad / 1000) / r;

  var i = 0;

  for (i = 0; i <= 360; i += detail) {
    var brng = i * pi / 180;

    var pLat = Math.asin(Math.sin(_lat) * Math.cos(d) + Math.cos(_lat) * Math.sin(d) * Math.cos(brng));
    var pLng = ((_lng + Math.atan2(Math.sin(brng) * Math.sin(d) * Math.cos(_lat), Math.cos(d) - Math.sin(_lat) * Math.sin(pLat))) * 180) / pi;
    pLat = (pLat * 180) / pi;

    staticMapSrc += "|" + pLat + "," + pLng;
  }

  return uri + encodeURI(staticMapSrc);
}

function zStringHasAlfaChars(str) {
  return str.match(/[a-z]/i);
}

function zUploadFile(file) {
  var url = 'server/index.php';
  var xhr = new XMLHttpRequest();
  var fd = new FormData();
  xhr.open("POST", url, true);
  xhr.onreadystatechange = function() {
    if (xhr.readyState == 4 && xhr.status == 200) {
      // Every thing ok, file uploaded
      console.log(xhr.responseText); // handle response.
    }
  };
  fd.append("upload_file", file);
  xhr.send(fd);
}

function zGetExtensionFromPath(path) {
  var i = path.lastIndexOf(".");
  if (i == -1) {
    return "";
  }
  return path.substring(i +1);
}

function zSetBrowserUrl(surl, title) {
    window.history.pushState("object", title, surl);
}

function zBase64ToUint8Array(base64) {
    var binary_string = window.atob(base64);
    var len = binary_string.length;
    var bytes = new Uint8Array(len);
    for (var i = 0; i < len; i++) {
        bytes[i] = binary_string.charCodeAt(i);
    }
    return new Uint8Array(bytes);
//    return bytes.buffer;
}

// CryptoJS v3.1.2
var CryptoJS=CryptoJS||function(s,p){var m={},l=m.lib={},n=function(){},r=l.Base={extend:function(b){n.prototype=this;var h=new n;b&&h.mixIn(b);h.hasOwnProperty("init")||(h.init=function(){h.$super.init.apply(this,arguments)});h.init.prototype=h;h.$super=this;return h},create:function(){var b=this.extend();b.init.apply(b,arguments);return b},init:function(){},mixIn:function(b){for(var h in b)b.hasOwnProperty(h)&&(this[h]=b[h]);b.hasOwnProperty("toString")&&(this.toString=b.toString)},clone:function(){return this.init.prototype.extend(this)}},
q=l.WordArray=r.extend({init:function(b,h){b=this.words=b||[];this.sigBytes=h!=p?h:4*b.length},toString:function(b){return(b||t).stringify(this)},concat:function(b){var h=this.words,a=b.words,j=this.sigBytes;b=b.sigBytes;this.clamp();if(j%4)for(var g=0;g<b;g++)h[j+g>>>2]|=(a[g>>>2]>>>24-8*(g%4)&255)<<24-8*((j+g)%4);else if(65535<a.length)for(g=0;g<b;g+=4)h[j+g>>>2]=a[g>>>2];else h.push.apply(h,a);this.sigBytes+=b;return this},clamp:function(){var b=this.words,h=this.sigBytes;b[h>>>2]&=4294967295<<
32-8*(h%4);b.length=s.ceil(h/4)},clone:function(){var b=r.clone.call(this);b.words=this.words.slice(0);return b},random:function(b){for(var h=[],a=0;a<b;a+=4)h.push(4294967296*s.random()|0);return new q.init(h,b)}}),v=m.enc={},t=v.Hex={stringify:function(b){var a=b.words;b=b.sigBytes;for(var g=[],j=0;j<b;j++){var k=a[j>>>2]>>>24-8*(j%4)&255;g.push((k>>>4).toString(16));g.push((k&15).toString(16))}return g.join("")},parse:function(b){for(var a=b.length,g=[],j=0;j<a;j+=2)g[j>>>3]|=parseInt(b.substr(j,
2),16)<<24-4*(j%8);return new q.init(g,a/2)}},a=v.Latin1={stringify:function(b){var a=b.words;b=b.sigBytes;for(var g=[],j=0;j<b;j++)g.push(String.fromCharCode(a[j>>>2]>>>24-8*(j%4)&255));return g.join("")},parse:function(b){for(var a=b.length,g=[],j=0;j<a;j++)g[j>>>2]|=(b.charCodeAt(j)&255)<<24-8*(j%4);return new q.init(g,a)}},u=v.Utf8={stringify:function(b){try{return decodeURIComponent(escape(a.stringify(b)))}catch(g){throw Error("Malformed UTF-8 data");}},parse:function(b){return a.parse(unescape(encodeURIComponent(b)))}},
g=l.BufferedBlockAlgorithm=r.extend({reset:function(){this._data=new q.init;this._nDataBytes=0},_append:function(b){"string"==typeof b&&(b=u.parse(b));this._data.concat(b);this._nDataBytes+=b.sigBytes},_process:function(b){var a=this._data,g=a.words,j=a.sigBytes,k=this.blockSize,m=j/(4*k),m=b?s.ceil(m):s.max((m|0)-this._minBufferSize,0);b=m*k;j=s.min(4*b,j);if(b){for(var l=0;l<b;l+=k)this._doProcessBlock(g,l);l=g.splice(0,b);a.sigBytes-=j}return new q.init(l,j)},clone:function(){var b=r.clone.call(this);
b._data=this._data.clone();return b},_minBufferSize:0});l.Hasher=g.extend({cfg:r.extend(),init:function(b){this.cfg=this.cfg.extend(b);this.reset()},reset:function(){g.reset.call(this);this._doReset()},update:function(b){this._append(b);this._process();return this},finalize:function(b){b&&this._append(b);return this._doFinalize()},blockSize:16,_createHelper:function(b){return function(a,g){return(new b.init(g)).finalize(a)}},_createHmacHelper:function(b){return function(a,g){return(new k.HMAC.init(b,
g)).finalize(a)}}});var k=m.algo={};return m}(Math);
(function(s){function p(a,k,b,h,l,j,m){a=a+(k&b|~k&h)+l+m;return(a<<j|a>>>32-j)+k}function m(a,k,b,h,l,j,m){a=a+(k&h|b&~h)+l+m;return(a<<j|a>>>32-j)+k}function l(a,k,b,h,l,j,m){a=a+(k^b^h)+l+m;return(a<<j|a>>>32-j)+k}function n(a,k,b,h,l,j,m){a=a+(b^(k|~h))+l+m;return(a<<j|a>>>32-j)+k}for(var r=CryptoJS,q=r.lib,v=q.WordArray,t=q.Hasher,q=r.algo,a=[],u=0;64>u;u++)a[u]=4294967296*s.abs(s.sin(u+1))|0;q=q.MD5=t.extend({_doReset:function(){this._hash=new v.init([1732584193,4023233417,2562383102,271733878])},
_doProcessBlock:function(g,k){for(var b=0;16>b;b++){var h=k+b,w=g[h];g[h]=(w<<8|w>>>24)&16711935|(w<<24|w>>>8)&4278255360}var b=this._hash.words,h=g[k+0],w=g[k+1],j=g[k+2],q=g[k+3],r=g[k+4],s=g[k+5],t=g[k+6],u=g[k+7],v=g[k+8],x=g[k+9],y=g[k+10],z=g[k+11],A=g[k+12],B=g[k+13],C=g[k+14],D=g[k+15],c=b[0],d=b[1],e=b[2],f=b[3],c=p(c,d,e,f,h,7,a[0]),f=p(f,c,d,e,w,12,a[1]),e=p(e,f,c,d,j,17,a[2]),d=p(d,e,f,c,q,22,a[3]),c=p(c,d,e,f,r,7,a[4]),f=p(f,c,d,e,s,12,a[5]),e=p(e,f,c,d,t,17,a[6]),d=p(d,e,f,c,u,22,a[7]),
c=p(c,d,e,f,v,7,a[8]),f=p(f,c,d,e,x,12,a[9]),e=p(e,f,c,d,y,17,a[10]),d=p(d,e,f,c,z,22,a[11]),c=p(c,d,e,f,A,7,a[12]),f=p(f,c,d,e,B,12,a[13]),e=p(e,f,c,d,C,17,a[14]),d=p(d,e,f,c,D,22,a[15]),c=m(c,d,e,f,w,5,a[16]),f=m(f,c,d,e,t,9,a[17]),e=m(e,f,c,d,z,14,a[18]),d=m(d,e,f,c,h,20,a[19]),c=m(c,d,e,f,s,5,a[20]),f=m(f,c,d,e,y,9,a[21]),e=m(e,f,c,d,D,14,a[22]),d=m(d,e,f,c,r,20,a[23]),c=m(c,d,e,f,x,5,a[24]),f=m(f,c,d,e,C,9,a[25]),e=m(e,f,c,d,q,14,a[26]),d=m(d,e,f,c,v,20,a[27]),c=m(c,d,e,f,B,5,a[28]),f=m(f,c,
d,e,j,9,a[29]),e=m(e,f,c,d,u,14,a[30]),d=m(d,e,f,c,A,20,a[31]),c=l(c,d,e,f,s,4,a[32]),f=l(f,c,d,e,v,11,a[33]),e=l(e,f,c,d,z,16,a[34]),d=l(d,e,f,c,C,23,a[35]),c=l(c,d,e,f,w,4,a[36]),f=l(f,c,d,e,r,11,a[37]),e=l(e,f,c,d,u,16,a[38]),d=l(d,e,f,c,y,23,a[39]),c=l(c,d,e,f,B,4,a[40]),f=l(f,c,d,e,h,11,a[41]),e=l(e,f,c,d,q,16,a[42]),d=l(d,e,f,c,t,23,a[43]),c=l(c,d,e,f,x,4,a[44]),f=l(f,c,d,e,A,11,a[45]),e=l(e,f,c,d,D,16,a[46]),d=l(d,e,f,c,j,23,a[47]),c=n(c,d,e,f,h,6,a[48]),f=n(f,c,d,e,u,10,a[49]),e=n(e,f,c,d,
C,15,a[50]),d=n(d,e,f,c,s,21,a[51]),c=n(c,d,e,f,A,6,a[52]),f=n(f,c,d,e,q,10,a[53]),e=n(e,f,c,d,y,15,a[54]),d=n(d,e,f,c,w,21,a[55]),c=n(c,d,e,f,v,6,a[56]),f=n(f,c,d,e,D,10,a[57]),e=n(e,f,c,d,t,15,a[58]),d=n(d,e,f,c,B,21,a[59]),c=n(c,d,e,f,r,6,a[60]),f=n(f,c,d,e,z,10,a[61]),e=n(e,f,c,d,j,15,a[62]),d=n(d,e,f,c,x,21,a[63]);b[0]=b[0]+c|0;b[1]=b[1]+d|0;b[2]=b[2]+e|0;b[3]=b[3]+f|0},_doFinalize:function(){var a=this._data,k=a.words,b=8*this._nDataBytes,h=8*a.sigBytes;k[h>>>5]|=128<<24-h%32;var l=s.floor(b/
4294967296);k[(h+64>>>9<<4)+15]=(l<<8|l>>>24)&16711935|(l<<24|l>>>8)&4278255360;k[(h+64>>>9<<4)+14]=(b<<8|b>>>24)&16711935|(b<<24|b>>>8)&4278255360;a.sigBytes=4*(k.length+1);this._process();a=this._hash;k=a.words;for(b=0;4>b;b++)h=k[b],k[b]=(h<<8|h>>>24)&16711935|(h<<24|h>>>8)&4278255360;return a},clone:function(){var a=t.clone.call(this);a._hash=this._hash.clone();return a}});r.MD5=t._createHelper(q);r.HmacMD5=t._createHmacHelper(q)})(Math);


