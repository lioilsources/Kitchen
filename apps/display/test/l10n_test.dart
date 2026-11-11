import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:predavka_display/l10n/gen/app_localizations.dart';

/// Šablona je app_cs.arb; každý další jazyk musí mít stejné klíče a placeholdery.
Map<String, dynamic> _arb(String path) => jsonDecode(File(path).readAsStringSync()) as Map<String, dynamic>;

Set<String> _placeholders(String text) => RegExp(r'\{(\w+)\}').allMatches(text).map((m) => m.group(1)!).toSet();

void main() {
  final template = _arb('lib/l10n/app_cs.arb');
  final keys = template.keys.where((k) => !k.startsWith('@')).toSet();
  final files = Directory('lib/l10n').listSync().whereType<File>().where((f) => f.path.endsWith('.arb')).toList();

  test('45 jazyků, všechny registrované', () {
    expect(files, hasLength(45));
    expect(AppLocalizations.supportedLocales, hasLength(45));
  });

  for (final f in files) {
    final name = f.uri.pathSegments.last;
    test('$name má všechny klíče a placeholdery', () {
      final arb = _arb(f.path);
      final own = arb.keys.where((k) => !k.startsWith('@')).toSet();
      expect(own.difference(keys), isEmpty, reason: 'klíče navíc');
      expect(keys.difference(own), isEmpty, reason: 'chybějící klíče');
      for (final k in keys.where((k) => k != '@@locale')) {
        final want = _placeholders(template[k] as String);
        final got = _placeholders(arb[k] as String);
        expect(got, equals(want), reason: k);
      }
    });
  }

  test('každý jazyk se načte a formátuje', () {
    for (final locale in AppLocalizations.supportedLocales) {
      final l = lookupAppLocalizations(locale);
      expect(l.etaInMin(6), contains('6'), reason: locale.toString());
      expect(l.modeKitchen, isNotEmpty);
    }
  });
}
