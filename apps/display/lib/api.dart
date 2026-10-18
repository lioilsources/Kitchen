import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';

import 'models.dart';

enum ConnState { connecting, connected, disconnected }

class ApiException implements Exception {
  ApiException(this.message);
  final String message;
  @override
  String toString() => message;
}

/// Klient serveru Předávka: WebSocket `/ws` (snapshoty, hlášky) + HTTP akce.
///
/// Auto-reconnect s exponenciálním backoffem (1 s → 15 s). Pokud déle než
/// [staleAfter] nepřijde žádná zpráva (server posílá heartbeat po 5 s),
/// je spojení považováno za ztracené – poslední data ale zůstávají.
class PredavkaClient extends ChangeNotifier {
  PredavkaClient(String baseUrl, {http.Client? httpClient})
    : baseUrl = normalizeBaseUrl(baseUrl),
      _http = httpClient ?? http.Client();

  static const staleAfter = Duration(seconds: 12);
  static const _connectTimeout = Duration(seconds: 8);
  static const _maxBackoff = Duration(seconds: 15);

  final String baseUrl;
  final http.Client _http;

  Snapshot? snapshot;
  Announce? lastAnnounce;

  /// Zvyšuje se s každou hláškou (i když je text stejný).
  int announceSeq = 0;

  ConnState state = ConnState.connecting;
  DateTime? lastMessageAt;
  bool stale = false;

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _sub;
  Timer? _reconnectTimer;
  Timer? _watchdog;
  int _attempt = 0;
  int _generation = 0;
  bool _disposed = false;
  final DateTime _startedAt = DateTime.now();
  final _rnd = Random();

  static String normalizeBaseUrl(String url) {
    var u = url.trim();
    if (u.isEmpty) return u;
    if (!u.contains('://')) u = 'http://$u';
    while (u.endsWith('/')) {
      u = u.substring(0, u.length - 1);
    }
    return u;
  }

  Uri get wsUri {
    final base = Uri.parse(baseUrl);
    final scheme = base.scheme == 'https' ? 'wss' : 'ws';
    final path = base.path.endsWith('/') ? '${base.path}ws' : '${base.path}/ws';
    return base.replace(scheme: scheme, path: path);
  }

  Uri _uri(String path) => Uri.parse('$baseUrl$path');

  void start() {
    _watchdog ??= Timer.periodic(const Duration(seconds: 1), (_) => _checkStale());
    _connect();
  }

  Future<void> _connect() async {
    if (_disposed) return;
    final gen = ++_generation;
    _setState(ConnState.connecting);
    try {
      final ch = WebSocketChannel.connect(wsUri);
      _channel = ch;
      await ch.ready.timeout(_connectTimeout);
      if (gen != _generation || _disposed) {
        ch.sink.close();
        return;
      }
      _attempt = 0;
      _setState(ConnState.connected);
      _sub = ch.stream.listen(
        _onData,
        onDone: () => _onClosed(gen),
        onError: (_) => _onClosed(gen),
        cancelOnError: true,
      );
    } catch (_) {
      _onClosed(gen);
    }
  }

  void _onData(dynamic raw) {
    if (raw is! String) return;
    ServerMessage? msg;
    try {
      msg = ServerMessage.fromJson(jsonDecode(raw));
    } catch (_) {
      return;
    }
    lastMessageAt = DateTime.now();
    stale = false;
    switch (msg) {
      case SnapshotMessage(:final snapshot):
        this.snapshot = snapshot;
      case AnnounceMessage(:final announce):
        if (announce.text.isNotEmpty) {
          lastAnnounce = announce;
          announceSeq++;
        }
      case ToneMessage():
      case null:
        break;
    }
    notifyListeners();
  }

  void _onClosed(int gen) {
    if (_disposed || gen != _generation) return;
    _generation++; // zneplatní další callbacky tohoto spojení
    _sub?.cancel();
    _sub = null;
    _channel?.sink.close();
    _channel = null;
    _setState(ConnState.disconnected);
    final secs = min(_maxBackoff.inSeconds, 1 << min(_attempt, 4));
    _attempt++;
    final delay = Duration(milliseconds: secs * 1000 + _rnd.nextInt(500));
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(delay, _connect);
  }

  void _checkStale() {
    final ref = lastMessageAt ?? _startedAt;
    final isStale = DateTime.now().difference(ref) > staleAfter;
    if (isStale != stale) {
      stale = isStale;
      notifyListeners();
    }
    // Spojení visí bez heartbeatu → zahodit a připojit znovu.
    if (isStale && state == ConnState.connected) {
      _onClosed(_generation);
    }
  }

  void _setState(ConnState s) {
    if (state == s) return;
    state = s;
    notifyListeners();
  }

  // ---------- HTTP ----------

  Future<Map<String, dynamic>?> _post(String path, [Object? body]) async {
    final http.Response res;
    try {
      res = await _http
          .post(
            _uri(path),
            headers: const {'Content-Type': 'application/json'},
            body: body == null ? null : jsonEncode(body),
          )
          .timeout(const Duration(seconds: 5));
    } on TimeoutException {
      throw ApiException('Server neodpovídá');
    } catch (_) {
      throw ApiException('Server není dostupný');
    }
    Object? decoded;
    try {
      decoded = res.bodyBytes.isEmpty ? null : jsonDecode(utf8.decode(res.bodyBytes));
    } catch (_) {
      decoded = null;
    }
    if (res.statusCode >= 400) {
      final err = decoded is Map ? decoded['error'] : null;
      throw ApiException(err is String && err.isNotEmpty ? err : 'Chyba ${res.statusCode}');
    }
    return decoded is Map ? decoded.cast<String, dynamic>() : null;
  }

  static String _seg(String s) => Uri.encodeComponent(s);

  Future<void> bumpItem(String id, {required String by}) => _post('/api/items/${_seg(id)}/bump', {'by': by});

  Future<void> bumpStation(String station, {required String by}) =>
      _post('/api/stations/${_seg(station)}/bump', {'by': by});

  Future<void> pickTable(String table) => _post('/api/tables/${_seg(table)}/pick');

  Future<void> fireTable(String table) => _post('/api/tables/${_seg(table)}/fire');

  Future<void> repeatAnnounce() => _post('/api/repeat');

  /// Jednorázové načtení stavu (např. seznam stanic v nastavení).
  static Future<Snapshot> fetchState(String baseUrl, {http.Client? client}) async {
    final c = client ?? http.Client();
    try {
      final res = await c.get(Uri.parse('${normalizeBaseUrl(baseUrl)}/api/state')).timeout(const Duration(seconds: 5));
      if (res.statusCode >= 400) throw ApiException('Chyba ${res.statusCode}');
      final decoded = jsonDecode(utf8.decode(res.bodyBytes));
      if (decoded is! Map) throw ApiException('Neplatná odpověď serveru');
      return Snapshot.fromJson(decoded.cast<String, dynamic>());
    } on ApiException {
      rethrow;
    } catch (_) {
      throw ApiException('Server není dostupný');
    } finally {
      if (client == null) c.close();
    }
  }

  @override
  void dispose() {
    _disposed = true;
    _generation++;
    _watchdog?.cancel();
    _reconnectTimer?.cancel();
    _sub?.cancel();
    _channel?.sink.close();
    _http.close();
    super.dispose();
  }
}
