import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'api.dart';
import 'screens/floor.dart';
import 'screens/kitchen.dart';
import 'screens/pass.dart';
import 'screens/settings.dart';
import 'settings.dart';
import 'widgets/announce_caption.dart';
import 'widgets/connection_overlay.dart';
import 'widgets/palette.dart';
import 'widgets/status_dot.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final settings = await AppSettings.load();
  runApp(PredavkaApp(initial: settings));
}

ThemeData buildTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: const Color(0xFF7FA6C9),
    brightness: Brightness.dark,
  ).copyWith(surface: Palette.background);
  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: Palette.background,
    canvasColor: Palette.background,
    splashFactory: InkSparkle.splashFactory,
  );
}

class PredavkaApp extends StatefulWidget {
  const PredavkaApp({super.key, required this.initial});

  final AppSettings initial;

  @override
  State<PredavkaApp> createState() => _PredavkaAppState();
}

class _PredavkaAppState extends State<PredavkaApp> {
  late AppSettings _settings = widget.initial;
  late bool _editing = !widget.initial.configured;
  PredavkaClient? _client;

  @override
  void initState() {
    super.initState();
    if (_settings.configured) _ensureClient();
  }

  void _ensureClient() {
    if (_client?.baseUrl == PredavkaClient.normalizeBaseUrl(_settings.serverUrl)) return;
    _client?.dispose();
    _client = PredavkaClient(_settings.serverUrl)..start();
  }

  Future<void> _onSave(AppSettings s) async {
    try {
      await s.save();
    } catch (_) {
      // Uložení selhalo (např. soukromé okno) – pokračujeme s nastavením v paměti.
    }
    setState(() {
      _settings = s;
      _editing = false;
      _ensureClient();
    });
  }

  @override
  void dispose() {
    _client?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final client = _client;
    return MaterialApp(
      title: 'Předávka',
      debugShowCheckedModeBanner: false,
      theme: buildTheme(),
      darkTheme: buildTheme(),
      themeMode: ThemeMode.dark,
      home: _editing || client == null
          ? SettingsScreen(
              initial: _settings,
              onSave: _onSave,
              onCancel: client == null ? null : () => setState(() => _editing = false),
            )
          : HomeScreen(
              key: ValueKey('${_settings.mode}/${_settings.station}/${client.baseUrl}'),
              client: client,
              settings: _settings,
              onOpenSettings: () => setState(() => _editing = true),
            ),
    );
  }
}

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key, required this.client, required this.settings, required this.onOpenSettings});

  final PredavkaClient client;
  final AppSettings settings;
  final VoidCallback onOpenSettings;

  @override
  Widget build(BuildContext context) {
    final body = switch (settings.mode) {
      DisplayMode.kitchen => KitchenScreen(client: client, settings: settings),
      DisplayMode.pass => PassScreen(client: client),
      DisplayMode.floor => FloorScreen(client: client),
    };
    return Focus(
      // Kuchyň má vlastní fokus pro bump bar (je potomkem, klávesy probublají sem).
      autofocus: settings.mode != DisplayMode.kitchen,
      // F2 / kontextové menu (dálkový ovladač) = nastavení.
      onKeyEvent: (node, e) {
        if (e is KeyDownEvent &&
            (e.logicalKey == LogicalKeyboardKey.f2 || e.logicalKey == LogicalKeyboardKey.contextMenu)) {
          onOpenSettings();
          return KeyEventResult.handled;
        }
        return KeyEventResult.ignored;
      },
      child: Scaffold(
        body: SafeArea(
          child: Column(
            children: [
              _TopBar(client: client, settings: settings, onOpenSettings: onOpenSettings),
              Expanded(
                child: Stack(
                  children: [
                    Positioned.fill(child: body),
                    Positioned(
                      left: 0,
                      right: 0,
                      bottom: 0,
                      child: Center(
                        child: AnnounceCaption(client: client, fontSize: settings.mode == DisplayMode.floor ? 40 : 26),
                      ),
                    ),
                    Positioned.fill(child: ConnectionOverlay(client: client)),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({required this.client, required this.settings, required this.onOpenSettings});

  final PredavkaClient client;
  final AppSettings settings;
  final VoidCallback onOpenSettings;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: client,
      builder: (context, _) {
        final s = client.snapshot;
        // Režim předávky se jmenuje stejně jako aplikace — nepsat to dvakrát.
        final parts = <String>[if (settings.mode != DisplayMode.pass) 'Předávka', settings.mode.label];
        if (settings.mode == DisplayMode.kitchen && settings.station.isNotEmpty) {
          parts.add(s?.stationName(settings.station) ?? settings.station);
          if (settings.cook.isNotEmpty) parts.add(settings.cook);
        }
        return Padding(
          padding: const EdgeInsets.fromLTRB(20, 10, 20, 10),
          child: Row(
            children: [
              GestureDetector(
                onLongPress: onOpenSettings,
                child: Text(
                  parts.join(' · '),
                  style: const TextStyle(fontSize: 20, color: Palette.textDim, fontWeight: FontWeight.w600),
                ),
              ),
              if (s?.quiet ?? false) ...[
                const SizedBox(width: 14),
                const Icon(Icons.volume_off_outlined, size: 20, color: Palette.textFaint),
              ],
              const Spacer(),
              _Clock(serverNow: s?.now),
              const SizedBox(width: 16),
              StatusDot(client: client),
            ],
          ),
        );
      },
    );
  }
}

/// Čas ze serveru (poslední snapshot), aby se displeje shodly i s rozladěnými hodinami tabletů.
class _Clock extends StatefulWidget {
  const _Clock({this.serverNow});

  final DateTime? serverNow;

  @override
  State<_Clock> createState() => _ClockState();
}

class _ClockState extends State<_Clock> {
  late Timer _t;
  DateTime _now = DateTime.now();

  @override
  void initState() {
    super.initState();
    _t = Timer.periodic(const Duration(seconds: 10), (_) => setState(() => _now = DateTime.now()));
  }

  @override
  void dispose() {
    _t.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final t = (widget.serverNow ?? _now).toLocal();
    final hh = t.hour.toString().padLeft(2, '0');
    final mm = t.minute.toString().padLeft(2, '0');
    return Text('$hh:$mm', style: const TextStyle(fontSize: 20, color: Palette.textDim));
  }
}
