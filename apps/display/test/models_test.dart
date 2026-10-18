import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:predavka_display/models.dart';
import 'package:predavka_display/screens/floor.dart';
import 'package:predavka_display/screens/pass.dart';
import 'package:predavka_display/widgets/format.dart';

const sampleSnapshot = '''
{
  "type": "snapshot",
  "data": {
    "now": "2026-10-18T12:00:00Z",
    "quiet": false,
    "stations": [{"id": "gril", "name": "Gril"}, {"id": "studena", "name": "Studená"}],
    "items": [
      {
        "id": "it_01J0001",
        "order_id": "or_01J0001",
        "table": "7",
        "sku": "svickova",
        "name": "Svíčková",
        "qty": 2,
        "mods": ["bez brusinek"],
        "note": "",
        "station": "gril",
        "course": 1,
        "status": "NEW",
        "claimed_by": "",
        "created_at": "2026-10-18T11:58:00Z",
        "claimed_at": null,
        "ready_at": null,
        "wait_s": 120,
        "eta_min": 9,
        "start_in_min": 0,
        "alert": "none"
      },
      {
        "id": "it_01J0002",
        "order_id": "or_01J0001",
        "table": "7",
        "sku": "salat",
        "name": "Salát",
        "qty": 1,
        "mods": [],
        "note": "rychle",
        "station": "studena",
        "course": 1,
        "status": "READY",
        "claimed_by": "Petr",
        "created_at": "2026-10-18T11:58:00Z",
        "claimed_at": "2026-10-18T11:58:30Z",
        "ready_at": "2026-10-18T11:59:00Z",
        "wait_s": 60,
        "eta_min": 0,
        "start_in_min": 0,
        "alert": "warn"
      },
      {
        "id": "it_01J0003",
        "order_id": "or_01J0002",
        "table": "3",
        "sku": "dort",
        "name": "Dort",
        "qty": 1,
        "mods": null,
        "note": "",
        "station": "studena",
        "course": 2,
        "status": "HOLD",
        "claimed_by": "",
        "created_at": "2026-10-18T11:50:00Z",
        "claimed_at": null,
        "ready_at": null,
        "wait_s": 600,
        "eta_min": 4,
        "start_in_min": 0,
        "alert": "none"
      }
    ],
    "tables": [
      {
        "table": "7",
        "status": "partial",
        "eta_min": 6,
        "course": 1,
        "items_total": 2,
        "items_ready": 1,
        "ready_wait_s": 60,
        "alert": "none",
        "items": [
          {"name": "Svíčková", "qty": 2, "status": "NEW"},
          {"name": "Salát", "qty": 1, "status": "READY"}
        ]
      },
      {
        "table": "3",
        "status": "hold",
        "eta_min": 0,
        "course": 2,
        "items_total": 1,
        "items_ready": 0,
        "ready_wait_s": 0,
        "alert": "none",
        "items": [{"name": "Dort", "qty": 1, "status": "HOLD"}]
      }
    ]
  }
}
''';

void main() {
  group('ServerMessage', () {
    test('parsuje snapshot podle protocol.md', () {
      final msg = ServerMessage.fromJson(jsonDecode(sampleSnapshot));
      expect(msg, isA<SnapshotMessage>());
      final s = (msg as SnapshotMessage).snapshot;

      expect(s.now, DateTime.utc(2026, 10, 18, 12));
      expect(s.quiet, isFalse);
      expect(s.stations.map((e) => e.id), ['gril', 'studena']);
      expect(s.stationName('studena'), 'Studená');
      expect(s.stationName('neznama'), 'neznama');

      expect(s.items, hasLength(3));
      final a = s.items[0];
      expect(a.id, 'it_01J0001');
      expect(a.orderId, 'or_01J0001');
      expect(a.table, '7');
      expect(a.sku, 'svickova');
      expect(a.name, 'Svíčková');
      expect(a.qty, 2);
      expect(a.mods, ['bez brusinek']);
      expect(a.station, 'gril');
      expect(a.course, 1);
      expect(a.status, ItemStatus.newItem);
      expect(a.createdAt, DateTime.utc(2026, 10, 18, 11, 58));
      expect(a.claimedAt, isNull);
      expect(a.readyAt, isNull);
      expect(a.waitS, 120);
      expect(a.etaMin, 9);
      expect(a.startInMin, 0);
      expect(a.alert, Alert.none);

      final b = s.items[1];
      expect(b.status, ItemStatus.ready);
      expect(b.claimedBy, 'Petr');
      expect(b.note, 'rychle');
      expect(b.readyAt, DateTime.utc(2026, 10, 18, 11, 59));
      expect(b.alert, Alert.warn);

      final c = s.items[2];
      expect(c.status, ItemStatus.hold);
      expect(c.mods, isEmpty); // null → prázdný seznam

      expect(s.tables, hasLength(2));
      final t = s.tables[0];
      expect(t.table, '7');
      expect(t.status, TableStatus.partial);
      expect(t.etaMin, 6);
      expect(t.itemsTotal, 2);
      expect(t.itemsReady, 1);
      expect(t.readyWaitS, 60);
      expect(t.alert, Alert.none);
      expect(t.items.map((i) => i.status), [ItemStatus.newItem, ItemStatus.ready]);
      expect(s.tables[1].status, TableStatus.hold);
    });

    test('parsuje announce a tone', () {
      final a = ServerMessage.fromJson(
        jsonDecode(
          '{"type":"announce","data":{"text":"Stůl sedm, kompletní.","kind":"complete","ts":"2026-10-18T12:00:00Z"}}',
        ),
      );
      expect(a, isA<AnnounceMessage>());
      final ann = (a as AnnounceMessage).announce;
      expect(ann.text, 'Stůl sedm, kompletní.');
      expect(ann.kind, 'complete');
      expect(ann.ts, DateTime.utc(2026, 10, 18, 12));

      final t = ServerMessage.fromJson(jsonDecode('{"type":"tone","data":{"kind":"claim"}}'));
      expect((t as ToneMessage).kind, 'claim');
    });

    test('toleruje neznámé a chybějící hodnoty', () {
      expect(ServerMessage.fromJson(jsonDecode('{"type":"nic"}')), isNull);
      expect(ServerMessage.fromJson('text'), isNull);
      final s = (ServerMessage.fromJson(jsonDecode('{"type":"snapshot","data":{}}')) as SnapshotMessage).snapshot;
      expect(s.items, isEmpty);
      expect(s.tables, isEmpty);
      expect(s.now, isNull);
      final i = ItemView.fromJson({'status': 'PICKED', 'alert': 'x', 'qty': '3'});
      expect(i.status, ItemStatus.unknown);
      expect(i.alert, Alert.none);
      expect(i.qty, 3);
    });
  });

  group('helpers', () {
    test('formátování', () {
      expect(etaText(6), 'za 6 min');
      expect(etaText(0), 'za chvíli');
      expect(durationText(30), '< 1 min');
      expect(durationText(125), '2 min');
      expect(qtyName(2, 'Svíčková'), '2× Svíčková');
      expect(qtyName(1, 'Salát'), 'Salát');
      expect(tableStatusLabel(TableStatus.ready), 'HOTOVO – vyzvedni');
    });

    test('předávka seskupí položky podle stolu', () {
      final s = (ServerMessage.fromJson(jsonDecode(sampleSnapshot)) as SnapshotMessage).snapshot;
      final g = groupForPass(s);
      expect(g.map((e) => e.table), ['7', '3']);
      expect(g[0].hasReady, isTrue);
      expect(g[0].items, hasLength(2));
      expect(g[1].hasHold, isTrue);
    });
  });

  testWidgets('FloorRow zobrazí stůl, ETA a stav', (tester) async {
    final s = (ServerMessage.fromJson(jsonDecode(sampleSnapshot)) as SnapshotMessage).snapshot;
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: FloorRow(table: s.tables[0])),
      ),
    );
    expect(find.text('7'), findsOneWidget);
    expect(find.text('za 6 min'), findsOneWidget);
    expect(find.text('částečně 1/2'), findsOneWidget);
    expect(find.text('2× Svíčková, Salát'), findsOneWidget);
  });
}
