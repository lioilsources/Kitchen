import 'package:flutter/material.dart';

import '../api.dart';
import '../models.dart';
import '../widgets/breathing.dart';
import '../widgets/format.dart';
import '../widgets/palette.dart';

/// Předávka (expo): stoly, co je hotové, co se ještě vaří.
/// Tap na stůl = vyzvednout READY položky, "Fire" = pustit další chod.
class PassScreen extends StatefulWidget {
  const PassScreen({super.key, required this.client});

  final PredavkaClient client;

  @override
  State<PassScreen> createState() => _PassScreenState();
}

class PassTableGroup {
  PassTableGroup(this.table, this.view);
  final String table;
  final TableView? view;
  final List<ItemView> items = [];

  Iterable<ItemView> get ready => items.where((i) => i.status == ItemStatus.ready);
  Iterable<ItemView> get hold => items.where((i) => i.status == ItemStatus.hold);
  bool get hasReady => ready.isNotEmpty;
  bool get hasHold => hold.isNotEmpty;
}

/// Seskupí položky podle stolu v pořadí `snapshot.tables` (ready nahoře).
List<PassTableGroup> groupForPass(Snapshot s) {
  final views = {for (final t in s.tables) t.table: t};
  final groups = <String, PassTableGroup>{};
  for (final it in s.items) {
    groups.putIfAbsent(it.table, () => PassTableGroup(it.table, views[it.table])).items.add(it);
  }
  final order = {for (var i = 0; i < s.tables.length; i++) s.tables[i].table: i};
  final list = groups.values.toList();
  list.sort((a, b) {
    if (a.hasReady != b.hasReady) return a.hasReady ? -1 : 1;
    final oa = order[a.table] ?? 1 << 20;
    final ob = order[b.table] ?? 1 << 20;
    return oa.compareTo(ob);
  });
  return list;
}

class _PassScreenState extends State<PassScreen> {
  final Set<String> _pending = {};

  Future<void> _run(String key, Future<void> Function() action) async {
    setState(() => _pending.add(key));
    try {
      await action();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
          ..hideCurrentSnackBar()
          ..showSnackBar(
            SnackBar(
              content: Text('Nepodařilo se: ${e.message}', style: const TextStyle(fontSize: 18)),
              backgroundColor: Palette.surfaceHigh,
            ),
          );
      }
    } finally {
      if (mounted) setState(() => _pending.remove(key));
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.client,
      builder: (context, _) {
        final s = widget.client.snapshot;
        if (s == null) {
          return const Center(
            child: Text('Připojuji se k serveru…', style: TextStyle(fontSize: 28, color: Palette.textDim)),
          );
        }
        final groups = groupForPass(s);
        if (groups.isEmpty) {
          return const Center(
            child: Text('Předávka je prázdná.', style: TextStyle(fontSize: 28, color: Palette.textDim)),
          );
        }
        return LayoutBuilder(
          builder: (context, c) {
            const gap = 16.0;
            final cols = (c.maxWidth / 420).floor().clamp(1, 5);
            final w = (c.maxWidth - 32 - gap * (cols - 1)) / cols;
            return SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 120),
              child: Wrap(
                spacing: gap,
                runSpacing: gap,
                children: [
                  for (final g in groups)
                    SizedBox(
                      width: w,
                      child: _PassCard(
                        key: ValueKey(g.table),
                        group: g,
                        stationName: s.stationName,
                        busy: _pending.contains('pick:${g.table}') || _pending.contains('fire:${g.table}'),
                        onPick: g.hasReady
                            ? () => _run('pick:${g.table}', () => widget.client.pickTable(g.table))
                            : null,
                        onFire: g.hasHold
                            ? () => _run('fire:${g.table}', () => widget.client.fireTable(g.table))
                            : null,
                      ),
                    ),
                ],
              ),
            );
          },
        );
      },
    );
  }
}

class _PassCard extends StatelessWidget {
  const _PassCard({
    super.key,
    required this.group,
    required this.stationName,
    required this.busy,
    required this.onPick,
    required this.onFire,
  });

  final PassTableGroup group;
  final String Function(String) stationName;
  final bool busy;
  final VoidCallback? onPick;
  final VoidCallback? onFire;

  @override
  Widget build(BuildContext context) {
    final v = group.view;
    final complete = v?.status == TableStatus.ready;
    final alert = Palette.alertColor(v?.alert ?? Alert.none);
    final base = complete ? Color.alphaBlend(Palette.ready.withValues(alpha: 0.16), Palette.surface) : Palette.surface;

    final header = Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Text(
          group.table,
          style: const TextStyle(fontSize: 60, height: 1, fontWeight: FontWeight.w800, color: Palette.text),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (v != null)
                Text(
                  tableStatusLabel(v.status),
                  style: TextStyle(
                    fontSize: 22,
                    fontWeight: FontWeight.w600,
                    color: complete ? Palette.ready : Palette.textDim,
                  ),
                ),
              if (v != null && !complete && v.status != TableStatus.hold)
                Text(etaText(v.etaMin), style: const TextStyle(fontSize: 20, color: Palette.textDim)),
              if (v != null && v.readyWaitS > 0)
                Text(
                  'na předávce ${durationText(v.readyWaitS)}',
                  style: TextStyle(fontSize: 18, color: alert ?? Palette.textDim),
                ),
            ],
          ),
        ),
        if (busy)
          const SizedBox(
            width: 22,
            height: 22,
            child: CircularProgressIndicator(strokeWidth: 2, color: Palette.textDim),
          ),
      ],
    );

    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: busy ? null : onPick,
        child: Breathing(
          color: alert,
          baseColor: base,
          child: Container(
            padding: const EdgeInsets.fromLTRB(20, 16, 20, 16),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(16),
              border: Border.all(
                color: complete ? Palette.ready.withValues(alpha: 0.6) : Palette.outline,
                width: complete ? 2 : 1.5,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                header,
                const SizedBox(height: 12),
                for (final it in group.items) _line(it),
                if (onPick != null || onFire != null) ...[
                  const SizedBox(height: 14),
                  Row(
                    children: [
                      if (onPick != null)
                        const Text('Klepni = vyzvednuto', style: TextStyle(fontSize: 16, color: Palette.textFaint)),
                      const Spacer(),
                      if (onFire != null)
                        FilledButton.tonal(
                          onPressed: busy ? null : onFire,
                          style: FilledButton.styleFrom(
                            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
                            textStyle: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700),
                          ),
                          child: const Text('Fire'),
                        ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _line(ItemView it) {
    final (color, right) = switch (it.status) {
      ItemStatus.ready => (Palette.ready, 'hotovo'),
      ItemStatus.hold => (Palette.textFaint, 'chod ${it.course}'),
      ItemStatus.newItem => (Palette.textDim, it.startInMin > 0 ? 'začít za ${it.startInMin} min' : 'čeká'),
      _ => (Palette.text, etaText(it.etaMin)),
    };
    final itemAlert = it.status == ItemStatus.hold ? null : Palette.alertColor(it.alert);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        children: [
          Container(
            width: 10,
            height: 10,
            margin: const EdgeInsets.only(right: 10),
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: it.status == ItemStatus.ready ? Palette.ready : Colors.transparent,
              border: Border.all(color: color, width: 2),
            ),
          ),
          Expanded(
            child: Text(
              qtyName(it.qty, it.name),
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 22, color: color, fontWeight: FontWeight.w500),
            ),
          ),
          const SizedBox(width: 8),
          Text(stationName(it.station), style: const TextStyle(fontSize: 16, color: Palette.textFaint)),
          const SizedBox(width: 12),
          Text(right, style: TextStyle(fontSize: 18, color: itemAlert ?? color)),
        ],
      ),
    );
  }
}
