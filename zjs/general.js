var token = "";
var map = {};
var mapMarker = null;
var geoLabelOrderXY = false;
var gRedColor = "#F66";
var gGreenColor = "#6C6";
var geocoder = {};

function setNowTimeToField(id) {
    var e = document.getElementById(id);
    var now = new Date();

    e.value = now.getFullString();
}

function setInfinityTimeToField(id) {
    var e = document.getElementById(id);
    e.value = "2200-01-01 00:00"
}

function setGuid(id) {
    var e = document.getElementById(id);
    e.value = makeRandomHex();
}

function setImageUrlToImageElement(id, url) {
    console.log("setImageUrlToImageElement");
    var e = document.getElementById(id);
    console.log("setImageUrl: " + url);
    e.src = url;
    if (url == "") {
        e.hidden = true;
    } else {
        e.hidden = false;
    }
    setNewValue("image", url);
}

function setValueFromField(key, val) {
    var f = parseFloat(val);
    if (!isNaN(f) && zIsStringNumericWithDot(val.trim())) {
        val = f;
    } else {
        val = val.trim();
    }
    setNewValue(key, val);
}

function setValueFromTextArea(tf, selid, event) {
    var sel = document.getElementById(selid);

    var c = (typeof event.which === "number") ? event.which : event.keyCode;
    if (c == zReturnKey) {
        setValueFromField(sel.value, tf.value);
        return sel.value; // key
    } else {
        if (setTextValueFromAlternatives(tf, event)) {
            return sel.value;
        }
    }
}

function setTextValueFromAlternatives(tf, event) {
    if (tf.alternatives != null && tf.alternatives.length > 1) {
        if (!tf.altIndex) {
            tf.altIndex = 0;
        }
        switch ((typeof event.which === "number") ? event.which : event.keyCode) {
            case zUpKey:
                if (tf.altIndex > 0) {
                    tf.altIndex--;
                }
                break;
            case zDownKey:
                if (tf.altIndex < tf.alternatives.length - 1) {
                    tf.altIndex++;
                }
                break;
            default:
                console.log("other key");
                return;
        }
        var a = tf.alternatives[tf.altIndex];
        tf.value = a;
        return true
    }
}

function setNewValue(key, val) {
    if (aceEditor) {
        var vals = getValues();
        var editor = ace.edit("valueseditor");

        vals[key] = val;
        svalues = JSON.stringify(vals, null, 2);
        editor.setValue(svalues)
    }
}

function getTimeFromField(id) {
    var val = document.getElementById(id).value;
    var timestamp = Date.parse(val);
    if (!isNaN(timestamp)) {
        var date = new Date(timestamp);
        return date.toJSON();
    }

    var str = val;
    var sdate = "";
    if (val == "") {
        return "0001-01-01 00:00";
    }
    var year = str.substr(0, 4);
    var month = str.substr(5, 2);
    var day = str.substr(8, 2);
    var hour = str.substr(11, 2);
    var min = str.substr(14, 2);

    var date = new Date(year, month - 1, day, hour, min, 0, 0);
    return date.toJSON();
}

function getValueFromField(id) {
    return document.getElementById(id).value;
}

function getDateOrZeroFromField(id) {
    var v = document.getElementById(id).value;
    if (v == null) {
        return "";
    }
    var date = new Date(v);
    return date.toJSON();
}

function getIntOrZeroFromField(id) {
    var v = parseInt(document.getElementById(id).value, 10);
    if (v == null) {
        return 0;
    }
    if (isNaN(v)) {
        return 0;
    }
    return v;
}

function getFloatOrZeroFromField(id) {
    var v = parseFloat(document.getElementById(id).value);
    if (v == null) {
        return 0.0;
    }
    if (isNaN(v)) {
        return 0.0;
    }
    return v;
}

function setToken(t) {
    token = t
    createCookie("token", token, 100);
}


function clearTextFieldSetPlaceNameFromSelection(id, selection) {
    var tf = document.getElementById(id);
    var ph = selection.options.item(selection.selectedIndex).getAttribute('ph');
    tf.placeholder = ph;
    tf.value = "";
    var salt = selection.options.item(selection.selectedIndex).getAttribute('alt');
    tf.alternatives = zGetSplitAndTrimmedString(salt, ",");
    tf.focus();
}

function markAndCenterMapOnAddress(map, address) {
    geocoder.geocode({
        'address': address
    }, function(results, status) {
        if (status === 'OK') {
            map.setCenter(results[0].geometry.location);
            var marker = new google.maps.Marker({
                map: map,
                position: results[0].geometry.location
            });
        } else {
            alert('Geocode was not successful for the following reason: ' + status);
        }
    });
}

function zSetFormValueFromRowId(row, id, formId) {
    var c = row.cells.namedItem(id).children;
    var val = c[c.length - 1];
    if (val.checked === true || val.checked === false) {
        console.log("val checked:", id, val.checked);
        document.getElementById(formId).checked = val.checked;
        return
    }
    if (val.length > 0) {
        val = val.children[0];
    }
    if (val.children.length > 0) {
        val = val.children[0];
    }
    //    console.log("zSetFormValueFromRowIdx:", val.length, val.children.length);
    val = val.innerHTML;

    //    console.log("zSetFormValueFromRowId:", val, id, formId, c[c.length - 1].children);
    document.getElementById(formId).value = val;
}

function zGetIndexOfElement(el) {
    var e = el.parentElement.parentElement
    var index = Array.prototype.slice.call(e.parentElement.children).indexOf(e)
    return index;
}

function postValue(url, name, value) {
    console.log("POSTVALUE:", url);
    if (Array.isArray(value) && value.length > 0) {
        console.log("postValue isArray:", name, value);
        value = value[0];
    }
    var m = [{
        "name": name,
        "time": (new Date()).toISOString(),
        "value": value
    }];
    var sjson = JSON.stringify(m);
    var info = { method: "POST", body: sjson };
    makeCorsRequest(url, info, function(obj, headers) {
        if (obj.url) {
            console.log("result:", obj.url);
        }
    }, function(code, err) {
        console.log("sendValue err:", code, err);
    });
}

function sendValue(url, name, value) {
    // console.log("SENDVAL:", name, value);
    if (Array.isArray(value) && value.length > 0) {
        console.log("sendValue isArray:", name, value);
        value = value[0];
    }
    url += "&name=" + name;
    url += "&time=" + (new Date()).toISOString();
    url += "&value=" + encodeURIComponent(value);

    var info = { method: "PUT" };
    makeCorsRequest(url, info, function(obj, headers) {
        //        console.log("sendValue4:", obj);
        if (obj.url) {
            console.log("result:", obj.url);
        }
    }, function(code, err) {
        console.log("sendValue err:", code, err);
    });
}