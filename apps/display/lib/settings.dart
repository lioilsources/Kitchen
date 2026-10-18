import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

enum DisplayMode {
  kitchen('Kuchyň'),
  pass('Předávka'),
  floor('Plac');

  const DisplayMode(this.label);
  final String label;

  static DisplayMode parse(String? v) => switch (v) {
    'floor' => DisplayMode.floor,
    'pass' => DisplayMode.pass,
    _ => DisplayMode.kitchen,
  };
}

/// Nastavení displeje (uloženo lokálně v zařízení).
class AppSettings {
  const AppSettings({
    required this.serverUrl,
    required this.mode,
    required this.station,
    required this.cook,
    required this.configured,
  });

  final String serverUrl;
  final DisplayMode mode;
  final String station;
  final String cook;

  /// false = první spuštění, ukáže se nastavení.
  final bool configured;

  static const fallbackUrl = 'http://predavka.local:8080';

  /// Na webu je výchozí server ten, ze kterého se stránka načetla.
  static String get defaultUrl {
    if (kIsWeb) {
      final base = Uri.base;
      if (base.scheme == 'http' || base.scheme == 'https') return base.origin;
    }
    return fallbackUrl;
  }

  /// Kdo bumpuje: jméno kuchaře, jinak stanice.
  String get bumpBy => cook.trim().isNotEmpty ? cook.trim() : station;

  static const _kUrl = 'server_url';
  static const _kMode = 'mode';
  static const _kStation = 'station';
  static const _kCook = 'cook';
  static const _kConfigured = 'configured';

  static Future<AppSettings> load() async {
    try {
      final p = await SharedPreferences.getInstance();
      return AppSettings(
        serverUrl: p.getString(_kUrl) ?? defaultUrl,
        mode: DisplayMode.parse(p.getString(_kMode)),
        station: p.getString(_kStation) ?? '',
        cook: p.getString(_kCook) ?? '',
        configured: p.getBool(_kConfigured) ?? false,
      );
    } catch (_) {
      return AppSettings(serverUrl: defaultUrl, mode: DisplayMode.kitchen, station: '', cook: '', configured: false);
    }
  }

  Future<void> save() async {
    final p = await SharedPreferences.getInstance();
    await p.setString(_kUrl, serverUrl);
    await p.setString(_kMode, mode.name);
    await p.setString(_kStation, station);
    await p.setString(_kCook, cook);
    await p.setBool(_kConfigured, true);
  }

  AppSettings copyWith({String? serverUrl, DisplayMode? mode, String? station, String? cook, bool? configured}) =>
      AppSettings(
        serverUrl: serverUrl ?? this.serverUrl,
        mode: mode ?? this.mode,
        station: station ?? this.station,
        cook: cook ?? this.cook,
        configured: configured ?? this.configured,
      );
}
