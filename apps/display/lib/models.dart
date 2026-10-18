// Datové modely podle docs/protocol.md (snapshot ze serveru predavka-server).

int _int(Object? v) {
  if (v is num) return v.toInt();
  if (v is String) return int.tryParse(v) ?? 0;
  return 0;
}

String _str(Object? v) => v == null ? '' : '$v';

List<String> _strList(Object? v) => v is List ? v.map((e) => '$e').toList(growable: false) : const [];

DateTime? _date(Object? v) => v is String && v.isNotEmpty ? DateTime.tryParse(v) : null;

List<Map<String, dynamic>> _objList(Object? v) =>
    v is List ? v.whereType<Map>().map((e) => e.cast<String, dynamic>()).toList() : const [];

enum Alert {
  none,
  warn,
  late;

  static Alert parse(Object? v) => switch (v) {
    'warn' => Alert.warn,
    'late' => Alert.late,
    _ => Alert.none,
  };
}

enum ItemStatus {
  hold,
  newItem,
  claimed,
  cooking,
  ready,
  unknown;

  static ItemStatus parse(Object? v) => switch (v) {
    'HOLD' => ItemStatus.hold,
    'NEW' => ItemStatus.newItem,
    'CLAIMED' => ItemStatus.claimed,
    'COOKING' => ItemStatus.cooking,
    'READY' => ItemStatus.ready,
    _ => ItemStatus.unknown,
  };

  /// Právě se na tom pracuje (převzato nebo se vaří).
  bool get inProgress => this == claimed || this == cooking;
}

enum TableStatus {
  waiting,
  cooking,
  partial,
  ready,
  hold,
  unknown;

  static TableStatus parse(Object? v) => switch (v) {
    'waiting' => TableStatus.waiting,
    'cooking' => TableStatus.cooking,
    'partial' => TableStatus.partial,
    'ready' => TableStatus.ready,
    'hold' => TableStatus.hold,
    _ => TableStatus.unknown,
  };
}

class Station {
  const Station({required this.id, required this.name});

  final String id;
  final String name;

  factory Station.fromJson(Map<String, dynamic> j) {
    final id = _str(j['id']);
    final name = _str(j['name']);
    return Station(id: id, name: name.isEmpty ? id : name);
  }
}

class ItemView {
  const ItemView({
    required this.id,
    required this.orderId,
    required this.table,
    required this.sku,
    required this.name,
    required this.qty,
    required this.mods,
    required this.note,
    required this.station,
    required this.course,
    required this.status,
    required this.claimedBy,
    required this.createdAt,
    required this.claimedAt,
    required this.readyAt,
    required this.waitS,
    required this.etaMin,
    required this.startInMin,
    required this.alert,
  });

  final String id;
  final String orderId;
  final String table;
  final String sku;
  final String name;
  final int qty;
  final List<String> mods;
  final String note;
  final String station;
  final int course;
  final ItemStatus status;
  final String claimedBy;
  final DateTime? createdAt;
  final DateTime? claimedAt;
  final DateTime? readyAt;
  final int waitS;
  final int etaMin;
  final int startInMin;
  final Alert alert;

  factory ItemView.fromJson(Map<String, dynamic> j) => ItemView(
    id: _str(j['id']),
    orderId: _str(j['order_id']),
    table: _str(j['table']),
    sku: _str(j['sku']),
    name: _str(j['name']),
    qty: j['qty'] == null ? 1 : _int(j['qty']),
    mods: _strList(j['mods']),
    note: _str(j['note']),
    station: _str(j['station']),
    course: _int(j['course']),
    status: ItemStatus.parse(j['status']),
    claimedBy: _str(j['claimed_by']),
    createdAt: _date(j['created_at']),
    claimedAt: _date(j['claimed_at']),
    readyAt: _date(j['ready_at']),
    waitS: _int(j['wait_s']),
    etaMin: _int(j['eta_min']),
    startInMin: _int(j['start_in_min']),
    alert: Alert.parse(j['alert']),
  );
}

class TableItem {
  const TableItem({required this.name, required this.qty, required this.status});

  final String name;
  final int qty;
  final ItemStatus status;

  factory TableItem.fromJson(Map<String, dynamic> j) => TableItem(
    name: _str(j['name']),
    qty: j['qty'] == null ? 1 : _int(j['qty']),
    status: ItemStatus.parse(j['status']),
  );
}

class TableView {
  const TableView({
    required this.table,
    required this.status,
    required this.etaMin,
    required this.course,
    required this.itemsTotal,
    required this.itemsReady,
    required this.readyWaitS,
    required this.alert,
    required this.items,
  });

  final String table;
  final TableStatus status;
  final int etaMin;
  final int course;
  final int itemsTotal;
  final int itemsReady;
  final int readyWaitS;
  final Alert alert;
  final List<TableItem> items;

  factory TableView.fromJson(Map<String, dynamic> j) => TableView(
    table: _str(j['table']),
    status: TableStatus.parse(j['status']),
    etaMin: _int(j['eta_min']),
    course: _int(j['course']),
    itemsTotal: _int(j['items_total']),
    itemsReady: _int(j['items_ready']),
    readyWaitS: _int(j['ready_wait_s']),
    alert: Alert.parse(j['alert']),
    items: _objList(j['items']).map(TableItem.fromJson).toList(),
  );
}

class Snapshot {
  const Snapshot({
    required this.now,
    required this.quiet,
    required this.stations,
    required this.items,
    required this.tables,
  });

  final DateTime? now;
  final bool quiet;
  final List<Station> stations;
  final List<ItemView> items;
  final List<TableView> tables;

  static const empty = Snapshot(now: null, quiet: false, stations: [], items: [], tables: []);

  factory Snapshot.fromJson(Map<String, dynamic> j) => Snapshot(
    now: _date(j['now']),
    quiet: j['quiet'] == true,
    stations: _objList(j['stations']).map(Station.fromJson).toList(),
    items: _objList(j['items']).map(ItemView.fromJson).toList(),
    tables: _objList(j['tables']).map(TableView.fromJson).toList(),
  );

  String stationName(String id) {
    for (final s in stations) {
      if (s.id == id) return s.name;
    }
    return id;
  }
}

class Announce {
  const Announce({required this.text, required this.kind, required this.ts});

  final String text;
  final String kind;
  final DateTime? ts;

  factory Announce.fromJson(Map<String, dynamic> j) =>
      Announce(text: _str(j['text']), kind: _str(j['kind']), ts: _date(j['ts']));
}

/// Jedna zpráva z WebSocketu `/ws`.
sealed class ServerMessage {
  const ServerMessage();

  /// Vrací null pro neznámé/poškozené zprávy (klient je ignoruje).
  static ServerMessage? fromJson(Object? decoded) {
    if (decoded is! Map) return null;
    final j = decoded.cast<String, dynamic>();
    final data = j['data'];
    final d = data is Map ? data.cast<String, dynamic>() : <String, dynamic>{};
    return switch (j['type']) {
      'snapshot' => SnapshotMessage(Snapshot.fromJson(d)),
      'announce' => AnnounceMessage(Announce.fromJson(d)),
      'tone' => ToneMessage(_str(d['kind'])),
      _ => null,
    };
  }
}

class SnapshotMessage extends ServerMessage {
  const SnapshotMessage(this.snapshot);
  final Snapshot snapshot;
}

class AnnounceMessage extends ServerMessage {
  const AnnounceMessage(this.announce);
  final Announce announce;
}

class ToneMessage extends ServerMessage {
  const ToneMessage(this.kind);
  final String kind;
}
