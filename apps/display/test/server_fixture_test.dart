import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:predavka_display/models.dart';

// Snapshot zachycený ze skutečného predavka-serveru (ESC/POS bon + tablet, claim, ready).
void main() {
  test('parses a real server snapshot', () {
    final raw = File('test/fixtures/snapshot_server.json').readAsStringSync();
    final s = Snapshot.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    expect(s.stations, isNotEmpty);
    expect(s.items, isNotEmpty);
    expect(s.tables, isNotEmpty);
    expect(s.items.any((i) => i.mods.isNotEmpty), isTrue);
    expect(s.tables.map((t) => t.table), containsAll(['7', '12']));
  });
}
