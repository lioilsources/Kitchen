import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api.dart';
import '../models.dart';
import '../settings.dart';
import '../widgets/palette.dart';
import '../widgets/ticket_card.dart';

/// Kuchyňská fronta jedné stanice + ovládání bump barem (klávesnice).
class KitchenScreen extends StatefulWidget {
  const KitchenScreen({super.key, required this.client, required this.settings});

  final PredavkaClient client;
  final AppSettings settings;

  @override
  State<KitchenScreen> createState() => _KitchenScreenState();
}

class _KitchenScreenState extends State<KitchenScreen> {
  final _focus = FocusNode(debugLabel: 'bumpbar');
  String? _selectedId;
  String? _flashId;
  Timer? _flashTimer;

  @override
  void dispose() {
    _flashTimer?.cancel();
    _focus.dispose();
    super.dispose();
  }

  /// Hlavní lístky (vybíratelné čísly 1–9): aktivní ve frontě, pak HOLD.
  List<ItemView> _main(Snapshot s) {
    final mine = s.items.where((i) => i.station == widget.settings.station);
    final active = mine.where((i) => i.status == ItemStatus.newItem || i.status.inProgress);
    final hold = mine.where((i) => i.status == ItemStatus.hold);
    return [...active, ...hold];
  }

  List<ItemView> _ready(Snapshot s) =>
      s.items.where((i) => i.station == widget.settings.station && i.status == ItemStatus.ready).toList();

  void _flash(String id) {
    _flashTimer?.cancel();
    setState(() => _flashId = id);
    _flashTimer = Timer(const Duration(milliseconds: 900), () {
      if (mounted) setState(() => _flashId = null);
    });
  }

  Future<void> _run(Future<void> Function() action) async {
    try {
      await action();
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
        ..hideCurrentSnackBar()
        ..showSnackBar(
          SnackBar(
            content: Text('Nepodařilo se: ${e.message}', style: const TextStyle(fontSize: 18)),
            backgroundColor: Palette.surfaceHigh,
            duration: const Duration(seconds: 3),
          ),
        );
    }
  }

  void _bumpItem(ItemView item) {
    _flash(item.id);
    setState(() => _selectedId = null);
    _run(() => widget.client.bumpItem(item.id, by: widget.settings.bumpBy));
  }

  void _bumpSelectedOrOldest(List<ItemView> main) {
    final sel = _selectedIndex(main);
    if (sel != null) {
      _bumpItem(main[sel]);
      return;
    }
    // Bez výběru: nejstarší položka stanice – rozhoduje server.
    final oldest = main.where((i) => i.status != ItemStatus.hold).firstOrNull;
    if (oldest != null) _flash(oldest.id);
    _run(() => widget.client.bumpStation(widget.settings.station, by: widget.settings.bumpBy));
  }

  int? _selectedIndex(List<ItemView> main) {
    if (_selectedId == null) return null;
    final i = main.indexWhere((e) => e.id == _selectedId);
    return i < 0 ? null : i;
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent e) {
    if (e is! KeyDownEvent) return KeyEventResult.ignored;
    final s = widget.client.snapshot;
    if (s == null) return KeyEventResult.ignored;
    final main = _main(s);
    final k = e.logicalKey;

    final digit = _digit(k);
    if (digit != null) {
      if (digit <= main.length) setState(() => _selectedId = main[digit - 1].id);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.enter ||
        k == LogicalKeyboardKey.numpadEnter ||
        k == LogicalKeyboardKey.space ||
        k == LogicalKeyboardKey.select) {
      _bumpSelectedOrOldest(main);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.arrowRight || k == LogicalKeyboardKey.arrowLeft) {
      if (main.isEmpty) return KeyEventResult.handled;
      final cur = _selectedIndex(main);
      final delta = k == LogicalKeyboardKey.arrowRight ? 1 : -1;
      final next = cur == null ? (delta > 0 ? 0 : main.length - 1) : (cur + delta).clamp(0, main.length - 1);
      setState(() => _selectedId = main[next].id);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.keyR) {
      _run(widget.client.repeatAnnounce);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.escape) {
      setState(() => _selectedId = null);
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  static int? _digit(LogicalKeyboardKey k) {
    const digits = [
      LogicalKeyboardKey.digit1, LogicalKeyboardKey.digit2, LogicalKeyboardKey.digit3, //
      LogicalKeyboardKey.digit4, LogicalKeyboardKey.digit5, LogicalKeyboardKey.digit6,
      LogicalKeyboardKey.digit7, LogicalKeyboardKey.digit8, LogicalKeyboardKey.digit9,
    ];
    const numpad = [
      LogicalKeyboardKey.numpad1, LogicalKeyboardKey.numpad2, LogicalKeyboardKey.numpad3, //
      LogicalKeyboardKey.numpad4, LogicalKeyboardKey.numpad5, LogicalKeyboardKey.numpad6,
      LogicalKeyboardKey.numpad7, LogicalKeyboardKey.numpad8, LogicalKeyboardKey.numpad9,
    ];
    final a = digits.indexOf(k);
    if (a >= 0) return a + 1;
    final b = numpad.indexOf(k);
    if (b >= 0) return b + 1;
    return null;
  }

  @override
  Widget build(BuildContext context) {
    if (widget.settings.station.isEmpty) {
      return const _Centered('Vyberte stanici v nastavení (dlouhý stisk na název nahoře).');
    }
    return Focus(
      focusNode: _focus,
      autofocus: true,
      onKeyEvent: _onKey,
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onTap: () => _focus.requestFocus(),
        child: ListenableBuilder(
          listenable: widget.client,
          builder: (context, _) {
            final s = widget.client.snapshot;
            if (s == null) return const _Centered('Připojuji se k serveru…');
            final main = _main(s);
            final ready = _ready(s);
            if (_selectedId != null && main.every((i) => i.id != _selectedId)) {
              _selectedId = null; // vybraný lístek zmizel
            }
            if (main.isEmpty && ready.isEmpty) {
              return const _Centered('Fronta je prázdná. Klid.');
            }
            return LayoutBuilder(
              builder: (context, c) {
                const gap = 16.0;
                final cols = (c.maxWidth / 380).floor().clamp(1, 6);
                final w = (c.maxWidth - 32 - gap * (cols - 1)) / cols;
                return SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 120),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Wrap(
                        spacing: gap,
                        runSpacing: gap,
                        children: [
                          for (var i = 0; i < main.length; i++)
                            SizedBox(
                              width: w,
                              child: TicketCard(
                                key: ValueKey(main[i].id),
                                item: main[i],
                                index: i < 9 ? i + 1 : null,
                                selected: main[i].id == _selectedId,
                                flash: main[i].id == _flashId,
                                onTap: () => _bumpItem(main[i]),
                              ),
                            ),
                        ],
                      ),
                      if (ready.isNotEmpty) ...[
                        const SizedBox(height: 28),
                        const Text(
                          'Na předávce',
                          style: TextStyle(fontSize: 18, color: Palette.textFaint, fontWeight: FontWeight.w600),
                        ),
                        const SizedBox(height: 10),
                        Wrap(
                          spacing: 10,
                          runSpacing: 10,
                          children: [
                            for (final r in ready) ReadyChip(key: ValueKey(r.id), item: r, onTap: () => _bumpItem(r)),
                          ],
                        ),
                      ],
                    ],
                  ),
                );
              },
            );
          },
        ),
      ),
    );
  }
}

class _Centered extends StatelessWidget {
  const _Centered(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(32),
      child: Text(
        text,
        textAlign: TextAlign.center,
        style: const TextStyle(fontSize: 28, color: Palette.textDim),
      ),
    ),
  );
}
