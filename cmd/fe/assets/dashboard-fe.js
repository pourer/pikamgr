'use strict';

var dashboard = angular.module('dashboard-fe', ["highcharts-ng", "ui.bootstrap"]);

function genXAuth(name) {
    return sha256("Codis-XAuth-[" + name + "]").substring(0, 32);
}

function concatUrl(base, name) {
    if (name) {
        return encodeURI(base + "?forward=" + name);
    } else {
        return encodeURI(base);
    }
}

function padInt2Str(num, size) {
    var s = num + "";
    while (s.length < size) s = "0" + s;
    return s;
}

function toJsonHtml(obj) {
    var json = angular.toJson(obj, 4);
    json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    json = json.replace(/ /g, '&nbsp;');
    json = json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
        var cls = 'number';
        if (/^"/.test(match)) {
            if (/:$/.test(match)) {
                cls = 'key';
            } else {
                cls = 'string';
            }
        } else if (/true|false/.test(match)) {
            cls = 'boolean';
        } else if (/null/.test(match)) {
            cls = 'null';
        }
        return '<span class="' + cls + '">' + match + '</span>';
    });
    return json;
}

function humanSize(size) {
    if (size < 1024) {
        return size + " B";
    }
    size /= 1024;
    if (size < 1024) {
        return size.toFixed(2) + " KB";
    }
    size /= 1024;
    if (size < 1024) {
        return size.toFixed(2) + " MB";
    }
    size /= 1024;
    if (size < 1024) {
        return size.toFixed(2) + " GB";
    }
    size /= 1024;
    return size.toFixed(2) + " TB";
}

function processSentinels(codis_stats, group_stats, codis_name) {
    var ha = codis_stats.sentinels;
    var out_of_sync = false;
    var servers = [];
    if (ha.model != undefined) {
        if (ha.model.servers == undefined) {
            ha.model.servers = []
        }
        for (var i = 0; i < ha.model.servers.length; i ++) {
            var x = {server: ha.model.servers[i]};
            var s = ha.stats[x.server];
            if (!s) {
                x.status = "PENDING";
            } else if (s.timeout) {
                x.status = "TIMEOUT";
            } else if (s.error) {
                x.status = "ERROR";
            } else {
                x.masters = 0;
                x.masters_down = 0;
                x.slaves = 0;
                x.sentinels = 0;
                var masters = s.stats["sentinel_masters"];
                if (masters != undefined) {
                    for (var j = 0; j < masters; j ++) {
                        var record = s.stats["master" + j];
                        if (record != undefined) {
                            var pairs = record.split(",");
                            var dict = {};
                            for (var t = 0; t < pairs.length; t ++) {
                                var ss = pairs[t].split("=");
                                if (ss.length == 2) {
                                    dict[ss[0]] = ss[1];
                                }
                            }
                            var name = dict["name"];
                            if (name == undefined) {
                                continue;
                            }
                            if (name.lastIndexOf(codis_name) != 0) {
                                continue;
                            }
                            if (name.lastIndexOf("-") != codis_name.length) {
                                continue;
                            }
                            x.masters ++;
                            if (dict["status"] != "ok") {
                                x.masters_down ++;
                            }
                            x.slaves += parseInt(dict["slaves"]);
                            x.sentinels += parseInt(dict["sentinels"]);
                        }
                    }
                }
                x.status_text = "masters=" + x.masters;
                x.status_text += ",down=" + x.masters_down;
                var avg = 0;
                if (x.slaves == 0) {
                    avg = 0;
                } else {
                    avg = Number(x.slaves) / x.masters;
                }
                x.status_text += ",slaves=" + avg.toFixed(2);
                if (x.sentinels == 0) {
                    avg = 0;
                } else {
                    avg = Number(x.sentinels) / x.masters;
                }
                x.status_text += ",sentinels=" + avg.toFixed(2);        
            }
            servers.push(x);
        }
        out_of_sync = ha.model.outOfSync;
    }
    var masters = ha.masters;
    if (masters == undefined) {
        masters = {};
    }
    return {servers:servers, masters:masters, out_of_sync: out_of_sync}
}

function processHAProxys(codis_stats) {
    var gslb = codis_stats.gslbs;
    var servers = [];
	
	var g = gslb.models["haproxy"];
	if (g) {
		if (g.servers) {
			for (var i = 0; i < g.servers.length; i ++) {
				var x = {server: g.servers[i]};
				var s = gslb.stats[x.server];
				if (!s) {
					x.status = "PENDING";
				} else if (s.timeout) {
					x.status = "TIMEOUT";
				} else if (s.error) {
					x.status = "ERROR";
				} else {
					x.status_text = "OK"
				}
				servers.push(x);
			}
		}
	}

    return {servers: servers};
}

function processLVS(codis_stats) {
    var gslb = codis_stats.gslbs;
    var servers = [];
	
	var g = gslb.models["lvs"]||{};
	if (g) {
		if (g.servers) {
			for (var i = 0; i < g.servers.length; i ++) {
				var x = {server: g.servers[i]};
				var s = gslb.stats[x.server];
				if (!s) {
					x.status = "PENDING";
				} else if (s.timeout) {
					x.status = "TIMEOUT";
				} else if (s.error) {
					x.status = "ERROR";
				} else {
					x.status_text = "OK"
				}
				servers.push(x);
			}
		}
	}
   
    return {servers: servers};
}

function processTemplateFiles(codis_stats) {
    var tfs = codis_stats.template;
    var files = [];
	
	for (var i = 0; i < tfs.fileNames.length; i ++) {
        var x = {name: tfs.fileNames[i]};
		files.push(x)
	}
   
    return {files: files};
}

function alertAction(text, callback) {
    BootstrapDialog.show({
        title: "Warning !!",
        message: text,
        closable: true,
        buttons: [{
            label: "OK",
            cssClass: "btn-primary",
            action: function (dialog) {
                dialog.close();
                callback();
            },
        }, {
            label: "CANCEL",
            action: function(dialogItself){
                dialogItself.close();
            }
        }],
    });
}

function alertAction2(text, callback) {
    BootstrapDialog.show({
        title: "Warning !!",
        type: "type-danger",
        message: text,
        closable: true,
        buttons: [{
            label: "JUST DO IT",
            cssClass: "btn-danger",
            action: function (dialog) {
                dialog.close();
                callback();
            },
        }, {
            label: "CANCEL",
            action: function(dialogItself){
                dialogItself.close();
            }
        }],
    });
}

function alertAction3(text) {
    BootstrapDialog.show({
        title: "Warning !!",
        type: "type-danger",
        message: text,
        closable: true,
        buttons: [{
            label: "CANCEL",
            action: function(dialogItself){
                dialogItself.close();
            }
        }],
    });
}

function alertErrorResp(failedResp) {
    var text = "error response";
    if (failedResp.status != 1500 && failedResp.status != 800) {
        text = failedResp.data.toString();
    } else {
        text = toJsonHtml(failedResp.data);
    }
    BootstrapDialog.alert({
        title: "Error !!",
        type: "type-danger",
        closable: true,
        message: text,
    });
}

function isValidInput(text) {
    return text && text != "" && text != "NA";
}

function processGroupStats(codis_stats) {
    var group_array = codis_stats.group.models;
    var group_stats = codis_stats.group.stats;
    var dbkeyRegexp = new RegExp(".* keys")
    for (var i = 0; i < group_array.length; i++) {
        var g = group_array[i];
        if (g.promoting.state) {
            g.ispromoting = g.promoting.state != "";
            if (g.promoting.index) {
                g.ispromoting_index = g.promoting.index;
            } else {
                g.ispromoting_index = 0;
            }
        } else {
            g.ispromoting = false;
            g.ispromoting_index = -1;
        }
        g.canremove = (g.servers == null || g.servers.length == 0);
        for (var j = 0; j < g.servers.length; j++) {
            var x = g.servers[j];
            var s = group_stats[x.server];
            x.keys = [];
            x.memory = "NA";
            x.maxmem = "NA";
			x.dbsize = "NA"
			x.qps = "NA";
            x.master = "NA";
            if (j == 0) {
                x.master_expect = "NO:ONE";
            } else {
                x.master_expect = g.servers[0].server;
            }
            if (!s) {
                x.status = "PENDING";
            } else if (s.timeout) {
                x.status = "TIMEOUT";
            } else if (s.error) {
                x.status = "ERROR";
            } else {
                for (var field in s.stats) {
                    if (dbkeyRegexp.test(field)) {
                        x.keys.push(field+ ": " + s.stats[field]);
                    }
                }
                if (s.stats["used_memory"]) {
                    var v = parseInt(s.stats["used_memory"], 10);
                    x.memory = humanSize(v);
                }
                if (s.stats["maxmemory"]) {
                    var v = parseInt(s.stats["maxmemory"], 10);
                    if (v == 0) {
                        x.maxmem = "INF."
                    } else {
                        x.maxmem = humanSize(v);
                    }
                }
				if (s.stats["db_size"]) {
                    var v = parseInt(s.stats["db_size"], 10);
                    x.dbsize = humanSize(v);
                }
				if (s.stats["instantaneous_ops_per_sec"]) {
                    x.qps = parseInt(s.stats["instantaneous_ops_per_sec"], 10);
                }
                if (s.stats["master_addr"]) {
                    x.master = s.stats["master_addr"] + ":" + s.stats["master_link_status"];
                } else {
                    x.master = "NO:ONE";
                }
                if (j == 0) {
                    x.master_status = (x.master == "NO:ONE");
                } else {
                    x.master_status = (x.master == g.servers[0].server + ":up");
                }
            }
            if (g.ispromoting) {
                x.canremove = false;
                x.canpromote = false;
				x.canForceFullSync = false;
                x.ispromoting = (j == g.ispromoting_index);
            } else {
                x.canremove = (j != 0 || g.servers.length <= 1);
                x.canpromote = j != 0;
				x.canForceFullSync = j != 0;
                x.ispromoting = false;
            }
            x.server_text = x.server;
        }
    }
    return {group_array: group_array};
}

dashboard.config(['$interpolateProvider',
    function ($interpolateProvider) {
        $interpolateProvider.startSymbol('[[');
        $interpolateProvider.endSymbol(']]');
    }
]);

dashboard.config(['$httpProvider', function ($httpProvider) {
    $httpProvider.defaults.useXDomain = true;
    delete $httpProvider.defaults.headers.common['X-Requested-With'];
}]);

dashboard.controller('MainCodisCtrl', ['$scope', '$http', '$uibModal', '$timeout',
    function ($scope, $http, $uibModal, $timeout) {
        Highcharts.setOptions({
            global: {
                useUTC: false,
            },
            exporting: {
                enabled: false,
            },
        });

        $scope.refresh_interval = 3;

        $scope.resetOverview = function () {
            $scope.codis_name = "NA";
            $scope.codis_addr = "NA";
            $scope.codis_coord_name = "Coordinator";
            $scope.codis_coord_addr = "NA";
            $scope.group_array = [];
            $scope.sentinel_servers = [];
            $scope.sentinel_out_of_sync = false;
			$scope.haproxy_servers = [];
			$scope.lvs_servers = [];
			$scope.template_files = [];
        }
        $scope.resetOverview();

        $http.get('/list').then(function (resp) {
            $scope.codis_list = resp.data;
        });

        $scope.selectCodisInstance = function (selected) {
            if ($scope.codis_name == selected) {
                return;
            }
            $scope.resetOverview();
            $scope.codis_name = selected;
            var url = concatUrl("/topom", selected);
            $http.get(url).then(function (resp) {
                if ($scope.codis_name != selected) {
                    return;
                }
                var overview = resp.data;
                $scope.codis_addr = overview.model.adminAddr;
                $scope.codis_coord_name = "[" + String(overview.config.coordinator_name).charAt(0).toUpperCase() + String(overview.config.coordinator_name).slice(1) + "]";
                $scope.codis_coord_addr = overview.config.coordinator_addr;
                $scope.updateStats(overview.stats);
            });
        }

        $scope.updateStats = function (codis_stats) {
            var group_stats = processGroupStats(codis_stats);
            var sentinel = processSentinels(codis_stats, group_stats, $scope.codis_name);
			var haproxy = processHAProxys(codis_stats);
			var lvs = processLVS(codis_stats);
			var template = processTemplateFiles(codis_stats);

            var merge = function(obj1, obj2) {
                if (obj1 === null || obj2 === null) {
                    return obj2;
                }
                if (Array.isArray(obj1)) {
                    if (obj1.length != obj2.length) {
                        return obj2;
                    }
                    for (var i = 0; i < obj1.length; i ++) {
                        obj1[i] = merge(obj1[i], obj2[i]);
                    }
                    return obj1;
                }
                if (typeof obj1 === "object") {
                    for (var k in obj1) {
                        if (obj2[k] === undefined) {
                            delete obj1[k];
                        }
                    }
                    for (var k in obj2) {
                        obj1[k] = merge(obj1[k], obj2[k]);
                    }
                    return obj1;
                }
                return obj2;
            }

            $scope.group_array = merge($scope.group_array, group_stats.group_array);
            $scope.sentinel_servers = merge($scope.sentinel_servers, sentinel.servers);
            $scope.sentinel_out_of_sync = sentinel.out_of_sync;
			$scope.haproxy_servers = merge($scope.haproxy_servers, haproxy.servers);
			$scope.lvs_servers = merge($scope.lvs_servers, lvs.servers);
			$scope.template_files = merge($scope.template_files, template.files);

            if ($scope.sentinel_servers.length != 0) {
                for (var i = 0; i < $scope.group_array.length; i ++) {
                    var g = $scope.group_array[i];
                    var ha_master = sentinel.masters[g.name];
                    var ha_master_ingroup = false;
                    for (var j = 0; j < g.servers.length; j ++) {
                        var x = g.servers[j];
                        if (ha_master == undefined) {
                            x.ha_status = "ha_undefined";
                            continue;
                        }
                        if (j == 0) {
                            if (x.server == ha_master) {
                                x.ha_status = "ha_master";
                            } else {
                                x.ha_status = "ha_not_master";
                            }
                        } else {
                            if (x.server == ha_master) {
                                x.ha_status = "ha_real_master";
                            } else {
                                x.ha_status = "ha_slave";
                            }
                        }
                        if (x.server == ha_master) {
                            x.server_text = x.server + " [HA]";
                            ha_master_ingroup = true;
                        }
                    }
                    if (ha_master == undefined || ha_master_ingroup) {
                        g.ha_warning = "";
                    } else {
                        g.ha_warning = "[HA: " + ha_master + "]";
                    }
                }
            }
        }

        $scope.refreshStats = function () {
            var codis_name = $scope.codis_name;
            var codis_addr = $scope.codis_addr;
            if (isValidInput(codis_name) && isValidInput(codis_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/stats/" + xauth, codis_name);
                $http.get(url).then(function (resp) {
                    if ($scope.codis_name != codis_name) {
                        return;
                    }
                    $scope.updateStats(resp.data);
                });
            }
        }

        $scope.createGroup = function (group_name, proxy_read_port, proxy_write_port) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(group_name) && isValidInput(proxy_read_port) && isValidInput(proxy_write_port)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/group/create/" + xauth + "/" + group_name + "/" + proxy_read_port + "/" + proxy_write_port, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.removeGroup = function (group_name) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/group/remove/" + xauth + "/" + group_name, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.resyncGroup = function (group) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var o = {};
                o.name = group.name;
                o.servers = [];
                var ha_real_master = -1;
                for (var j = 0; j < group.servers.length; j++) {
                    o.servers.push(group.servers[j].server);
                    if (group.servers[j].ha_status == "ha_real_master") {
                        ha_real_master = j;
                    }
                }
                if (ha_real_master < 0) {
                    alertAction("Resync Group-[" + group.name + "]: " + toJsonHtml(o), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync/" + xauth + "/" + group.name, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(o);
                    prompts += "\n\n";
                    prompts += "HA: server[" + ha_real_master + "]=" + group.servers[ha_real_master].server + " should be the real group master, do you really want to resync group-[" + group.name + "] ??";
                    alertAction2("Resync Group-[" + group.name + "]: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync/" + xauth + "/" + group.name, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.resyncGroupAll = function() {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var ha_real_master = -1;
                var gNames = [];
                for (var i = 0; i < $scope.group_array.length; i ++) {
                    var group = $scope.group_array[i];
                    for (var j = 0; j < group.servers.length; j++) {
                        if (group.servers[j].ha_status == "ha_real_master") {
                            ha_real_master = j;
                        }
                    }
                    gNames.push(group.name);
                }
                if (ha_real_master < 0) {
                    alertAction("Resync All Groups: group-[" + gNames + "]", function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    alertAction2("Resync All Groups: group-[" + gNames + "] (in conflict with HA)", function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.resyncSentinels = function () {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var servers = [];
                for (var i = 0; i < $scope.sentinel_servers.length; i ++) {
                    servers.push($scope.sentinel_servers[i].server);
                }
                var confused = [];
                for (var i = 0; i < $scope.group_array.length; i ++) {
                    var group = $scope.group_array[i];
                    var ha_real_master = -1;
                    for (var j = 0; j < group.servers.length; j ++) {
                        if (group.servers[j].ha_status == "ha_real_master") {
                            ha_real_master = j;
                        }
                    }
                    if (ha_real_master >= 0) {
                        confused.push({group: group.name, logical_master: group.servers[0].server, ha_real_master: group.servers[ha_real_master].server});
                    }
                }
                if (confused.length == 0) {
                    alertAction("Resync All Sentinels: " + toJsonHtml(servers), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/sentinels/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(servers);
                    prompts += "\n\n";
                    prompts += "HA: real master & logical master area conflicting: " + toJsonHtml(confused);
                    prompts += "\n\n";
                    prompts += "Please fix these before resync sentinels.";
                    alertAction2("Resync All Sentinels: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/sentinels/resync-all/" + xauth, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

        $scope.addSentinel = function (server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/sentinels/add/" + xauth + "/" + server_addr, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }
		
        $scope.delSentinel = function (sentinel, force) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                var prefix = "";
                if (force) {
                    prefix = "[FORCE] ";
                }
                alertAction(prefix + "Remove sentinel " + sentinel.server, function () {
                    var xauth = genXAuth(codis_name);
                    var value = 0;
                    if (force) {
                        value = 1;
                    }
                    var url = concatUrl("/api/topom/sentinels/del/" + xauth + "/" + sentinel.server + "/" + value, codis_name);
                    $http.put(url).then(function () {
                        $scope.refreshStats();
                    }, function (failedResp) {
                        alertErrorResp(failedResp);
                    });
                });
            }
        }

		$scope.addHAProxy = function (server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/gslbs/add/" + xauth + "/haproxy/" + server_addr, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }
		
		$scope.delHAProxy = function (haprxoy) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                alertAction("Remove haprxoy " + haprxoy.server, function () {
                    var xauth = genXAuth(codis_name);
                    var url = concatUrl("/api/topom/gslbs/del/" + xauth + "/haproxy/" + haprxoy.server, codis_name);
                    $http.put(url).then(function () {
                        $scope.refreshStats();
                    }, function (failedResp) {
                        alertErrorResp(failedResp);
                    });
                });
            }
        }
		
		$scope.addLVS = function (server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                var url = concatUrl("/api/topom/gslbs/add/" + xauth + "/lvs/" + server_addr, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }
		
		$scope.delLVS = function (lvs) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name)) {
                alertAction("Remove lvs " + lvs.server, function () {
                    var xauth = genXAuth(codis_name);
                    var url = concatUrl("/api/topom/gslbs/del/" + xauth + "/lvs/" + lvs.server, codis_name);
                    $http.put(url).then(function () {
                        $scope.refreshStats();
                    }, function (failedResp) {
                        alertErrorResp(failedResp);
                    });
                });
            }
        }
		
        $scope.addGroupServer = function (group_name, datacenter, server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(group_name) && isValidInput(server_addr)) {
                var xauth = genXAuth(codis_name);
                if (datacenter == undefined) {
                    datacenter = "";
                } else {
                    datacenter = datacenter.trim();
                }
                var suffix = "";
                if (datacenter != "") {
                    suffix = "/" + datacenter;
                }
                var url = concatUrl("/api/topom/group/add/" + xauth + "/" + group_name + "/" + server_addr + suffix, codis_name);
                $http.put(url).then(function () {
                    $scope.refreshStats();
                }, function (failedResp) {
                    alertErrorResp(failedResp);
                });
            }
        }

        $scope.delGroupServer = function (group, server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var o = {};
                o.name = group.name;
                o.servers = [];
                var ha_real_master = -1;
                for (var j = 0; j < group.servers.length; j++) {
                    o.servers.push(group.servers[j].server);
                    if (group.servers[j].ha_status == "ha_real_master") {
                        ha_real_master = j;
                    }
                }
                if (ha_real_master < 0 || group.servers[ha_real_master].server != server_addr) {
                    alertAction("Remove server " + server_addr + " from Group-[" + group.name + "]: " + toJsonHtml(o), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/del/" + xauth + "/" + group.name + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(o);
                    prompts += "\n\n";
                    prompts += "HA: server[" + ha_real_master + "]=" + server_addr + " should be the real group master, do you really want to remove it ??";
                    alertAction2("Remove server " + server_addr + " from Group-[" + group.name + "]: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/del/" + xauth + "/" + group.name + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });

                }
            }
        }
		
		$scope.promoteServer = function (group, server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
                var o = {};
                o.name = group.name;
                o.servers = [];
                var ha_real_master = -1;
                for (var j = 0; j < group.servers.length; j++) {
                    o.servers.push(group.servers[j].server);
                    if (group.servers[j].ha_status == "ha_real_master") {
                        ha_real_master = j;
                    }
                }
                if (ha_real_master < 0 || group.servers[ha_real_master].server == server_addr) {
                    alertAction("Promote server " + server_addr + " from Group-[" + group.name + "]: " + toJsonHtml(o), function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/promote/" + xauth + "/" + group.name + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                } else {
                    var prompts = toJsonHtml(o);
                    prompts += "\n\n";
                    prompts += "HA: server[" + ha_real_master + "]=" + group.servers[ha_real_master].server + " should be the real group master, do you really want to promote " + server_addr + " ??";
                    alertAction2("Promote server " + server_addr + " from Group-[" + group.name + "]: " + prompts, function () {
                        var xauth = genXAuth(codis_name);
                        var url = concatUrl("/api/topom/group/promote/" + xauth + "/" + group.name + "/" + server_addr, codis_name);
                        $http.put(url).then(function () {
                            $scope.refreshStats();
                        }, function (failedResp) {
                            alertErrorResp(failedResp);
                        });
                    });
                }
            }
        }

		$scope.createForceFullSyncAction = function (group, server_addr) {
            var codis_name = $scope.codis_name;
            if (isValidInput(codis_name) && isValidInput(server_addr)) {
				var o = {};
                o.name = group.name;
                o.servers = [];
                var ha_master = -1;
                for (var j = 0; j < group.servers.length; j++) {
                    o.servers.push(group.servers[j].server);
                    if (j == 0) {
                        ha_master = j;
                    }
                }
				
				if (ha_master < 0) {
					alertAction3("Find not master server in Group-[" + group.name + "]: " + toJsonHtml(o))
				}else if (group.servers[ha_master].server == server_addr) {
					alertAction3("Master server: " + server_addr + " not allowed this operation in Group-[" + group.name + "]: " + toJsonHtml(o))
				}else {
					var prompts = "ForceFullSync server: " + server_addr + " from master: " + group.servers[ha_master].server + " in Group-[" + group.name + "]: " + toJsonHtml(o);
                    prompts += "\n\n";
					prompts += "Do you really want to ForceFullSync server: " + server_addr + " from master: " + group.servers[ha_master].server;
					alertAction2(prompts, function () {
                        var xauth = genXAuth(codis_name);
						var url = concatUrl("/api/topom/group/force-full-sync/" + xauth + "/" + group.name + "/" + server_addr, codis_name);
						$http.put(url).then(function () {
							$scope.refreshStats();
						}, function (failedResp) {
							alertErrorResp(failedResp);
						});
                    });
				}
            }
        }
		
        if (window.location.hash) {
            $scope.selectCodisInstance(window.location.hash.substring(1));
        }

        var ticker = 0;
        (function autoRefreshStats() {
            if (ticker >= $scope.refresh_interval) {
                ticker = 0;
                $scope.refreshStats();
            }
            ticker++;
            $timeout(autoRefreshStats, 1000);
        }());
    }
])
;
