import '../models.dart';

/// Celé minuty (dolů) ze sekund.
int minutesOf(int seconds) => seconds <= 0 ? 0 : seconds ~/ 60;

/// "za 6 min" / "za chvíli".
String etaText(int etaMin) => etaMin <= 0 ? 'za chvíli' : 'za $etaMin min';

/// Krátká délka: "3 min" / "< 1 min".
String durationText(int seconds) {
  final m = minutesOf(seconds);
  return m < 1 ? '< 1 min' : '$m min';
}

String itemStatusLabel(ItemStatus s) => switch (s) {
  ItemStatus.hold => 'čeká na chod',
  ItemStatus.newItem => 'nové',
  ItemStatus.claimed => 'převzato',
  ItemStatus.cooking => 'vaří se',
  ItemStatus.ready => 'hotovo',
  ItemStatus.unknown => '',
};

String tableStatusLabel(TableStatus s) => switch (s) {
  TableStatus.waiting => 've frontě',
  TableStatus.cooking => 'vaří se',
  TableStatus.partial => 'částečně',
  TableStatus.ready => 'HOTOVO – vyzvedni',
  TableStatus.hold => 'čeká na chod',
  TableStatus.unknown => '',
};

String qtyName(int qty, String name) => qty > 1 ? '$qty× $name' : name;
