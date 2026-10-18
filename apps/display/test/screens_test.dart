import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:predavka_display/api.dart';
import 'package:predavka_display/models.dart';
import 'package:predavka_display/screens/floor.dart';
import 'package:predavka_display/screens/kitchen.dart';
import 'package:predavka_display/screens/pass.dart';
import 'package:predavka_display/settings.dart';

import 'models_test.dart' show sampleSnapshot;

PredavkaClient _client() {
  final c = PredavkaClient('http://localhost:1');
  c.snapshot = (ServerMessage.fromJson(jsonDecode(sampleSnapshot)) as SnapshotMessage).snapshot;
  return c;
}

const _settings = AppSettings(
  serverUrl: 'http://localhost:1',
  mode: DisplayMode.kitchen,
  station: 'gril',
  cook: '',
  configured: true,
);

Future<void> _pump(WidgetTester tester, Widget w, Size size) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(MaterialApp(home: Scaffold(body: w)));
  await tester.pump(const Duration(milliseconds: 500));
}

void main() {
  for (final size in const [Size(1920, 1080), Size(400, 800)]) {
    testWidgets('kuchyň ${size.width}', (tester) async {
      final c = _client();
      addTearDown(c.dispose);
      await _pump(tester, KitchenScreen(client: c, settings: _settings), size);
      expect(find.text('2× Svíčková'), findsOneWidget);
      expect(find.text('bez brusinek'), findsOneWidget);
      expect(find.text('Dort'), findsNothing); // jiná stanice
    });

    testWidgets('předávka ${size.width}', (tester) async {
      final c = _client();
      addTearDown(c.dispose);
      await _pump(tester, PassScreen(client: c), size);
      expect(find.text('Fire'), findsOneWidget);
      expect(find.text('Salát'), findsOneWidget);
    });

    testWidgets('plac ${size.width}', (tester) async {
      final c = _client();
      addTearDown(c.dispose);
      await _pump(tester, FloorScreen(client: c), size);
      expect(find.text('čeká na chod'), findsOneWidget);
    });
  }
}
