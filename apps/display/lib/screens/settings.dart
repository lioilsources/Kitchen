import 'package:flutter/material.dart';

import '../api.dart';
import '../l10n/gen/app_localizations.dart';
import '../models.dart';
import '../settings.dart';
import '../widgets/palette.dart';

/// Nastavení displeje: server, režim, stanice, jméno kuchaře.
class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key, required this.initial, required this.onSave, this.onCancel});

  final AppSettings initial;
  final ValueChanged<AppSettings> onSave;
  final VoidCallback? onCancel;

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  late final _url = TextEditingController(text: widget.initial.serverUrl);
  late final _cook = TextEditingController(text: widget.initial.cook);
  late final _stationText = TextEditingController(text: widget.initial.station);
  late DisplayMode _mode = widget.initial.mode;
  late String _station = widget.initial.station;

  List<Station>? _stations;
  ApiException? _error;
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _loadStations();
  }

  @override
  void dispose() {
    _url.dispose();
    _cook.dispose();
    _stationText.dispose();
    super.dispose();
  }

  Future<void> _loadStations() async {
    final url = _url.text;
    if (url.trim().isEmpty) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final s = await PredavkaClient.fetchState(url);
      if (!mounted) return;
      setState(() {
        _stations = s.stations;
        if (_station.isEmpty && s.stations.isNotEmpty) _station = s.stations.first.id;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _save() {
    final station = _stations != null && _stations!.isNotEmpty ? _station : _stationText.text.trim();
    widget.onSave(
      AppSettings(
        serverUrl: PredavkaClient.normalizeBaseUrl(_url.text.isEmpty ? AppSettings.defaultUrl : _url.text),
        mode: _mode,
        station: station,
        cook: _cook.text.trim(),
        configured: true,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    const label = TextStyle(fontSize: 18, color: Palette.textDim, fontWeight: FontWeight.w600);
    final stations = _stations;
    final needsStation = _mode == DisplayMode.kitchen;

    return Scaffold(
      appBar: AppBar(
        title: Text(l.settingsTitle),
        backgroundColor: Palette.background,
        leading: widget.onCancel == null
            ? null
            : IconButton(icon: const Icon(Icons.close), onPressed: widget.onCancel, tooltip: l.back),
      ),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 640),
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              Text(l.server, style: label),
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _url,
                      keyboardType: TextInputType.url,
                      autocorrect: false,
                      decoration: const InputDecoration(
                        border: OutlineInputBorder(),
                        hintText: AppSettings.fallbackUrl,
                      ),
                      onSubmitted: (_) => _loadStations(),
                    ),
                  ),
                  const SizedBox(width: 12),
                  OutlinedButton(
                    onPressed: _loading ? null : _loadStations,
                    child: _loading
                        ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                        : Text(l.loadStations),
                  ),
                ],
              ),
              if (_error != null) ...[
                const SizedBox(height: 8),
                Text(l.stationsLoadFailed(_error!.localized(l)), style: const TextStyle(color: Palette.warn)),
              ],
              const SizedBox(height: 28),
              Text(l.modeLabel, style: label),
              const SizedBox(height: 8),
              SegmentedButton<DisplayMode>(
                segments: [
                  ButtonSegment(
                    value: DisplayMode.kitchen,
                    label: Text(l.modeKitchen),
                    icon: const Icon(Icons.soup_kitchen_outlined),
                  ),
                  ButtonSegment(
                    value: DisplayMode.pass,
                    label: Text(l.modePass),
                    icon: const Icon(Icons.room_service_outlined),
                  ),
                  ButtonSegment(
                    value: DisplayMode.floor,
                    label: Text(l.modeFloor),
                    icon: const Icon(Icons.tv_outlined),
                  ),
                ],
                selected: {_mode},
                onSelectionChanged: (s) => setState(() => _mode = s.first),
              ),
              if (needsStation) ...[
                const SizedBox(height: 28),
                Text(l.stationLabel, style: label),
                const SizedBox(height: 8),
                if (stations != null && stations.isNotEmpty)
                  DropdownButtonFormField<String>(
                    initialValue: stations.any((s) => s.id == _station) ? _station : stations.first.id,
                    decoration: const InputDecoration(border: OutlineInputBorder()),
                    items: [
                      for (final s in stations) DropdownMenuItem(value: s.id, child: Text('${s.name}  (${s.id})')),
                    ],
                    onChanged: (v) => setState(() => _station = v ?? _station),
                  )
                else
                  TextField(
                    controller: _stationText,
                    autocorrect: false,
                    decoration: InputDecoration(border: const OutlineInputBorder(), hintText: l.stationExample),
                  ),
                const SizedBox(height: 28),
                Text(l.cookNameOptional, style: label),
                const SizedBox(height: 8),
                TextField(
                  controller: _cook,
                  decoration: InputDecoration(border: const OutlineInputBorder(), hintText: l.cookNameHint),
                ),
              ],
              const SizedBox(height: 36),
              FilledButton(
                onPressed: _save,
                style: FilledButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: 18),
                  textStyle: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700),
                ),
                child: Text(l.saveAndStart),
              ),
              const SizedBox(height: 16),
              Text(l.settingsHowTo, style: const TextStyle(color: Palette.textFaint)),
            ],
          ),
        ),
      ),
    );
  }
}
