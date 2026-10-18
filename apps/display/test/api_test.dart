import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:predavka_display/api.dart';

import 'models_test.dart' show sampleSnapshot;

void main() {
  test('URL helpery', () {
    expect(PredavkaClient.normalizeBaseUrl('predavka.local:8080/'), 'http://predavka.local:8080');
    expect(PredavkaClient('http://a:8080').wsUri.toString(), 'ws://a:8080/ws');
    expect(PredavkaClient('https://a').wsUri.toString(), 'wss://a/ws');
  });

  test('WS snapshot + announce a HTTP bump proti mock serveru', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final posts = <String, String>{};
    server.listen((req) async {
      if (req.uri.path == '/ws') {
        final ws = await WebSocketTransformer.upgrade(req);
        ws.add(jsonEncode(jsonDecode(sampleSnapshot)));
        ws.add(
          '{"type":"announce","data":{"text":"Stůl sedm, kompletní.","kind":"complete","ts":"2026-10-18T12:00:00Z"}}',
        );
      } else if (req.method == 'POST') {
        posts[req.uri.path] = await utf8.decodeStream(req);
        if (req.uri.path.contains('missing')) {
          req.response.statusCode = 404;
          req.response.write('{"error":"položka neexistuje"}');
        } else {
          req.response.write('{}');
        }
        await req.response.close();
      }
    });

    final c = PredavkaClient('http://127.0.0.1:${server.port}');
    final got = Completer<void>();
    c.addListener(() {
      if (c.snapshot != null && c.lastAnnounce != null && !got.isCompleted) got.complete();
    });
    c.start();
    await got.future.timeout(const Duration(seconds: 5));
    expect(c.state, ConnState.connected);
    expect(c.snapshot!.items, hasLength(3));
    expect(c.lastAnnounce!.text, 'Stůl sedm, kompletní.');
    expect(c.announceSeq, 1);

    await c.bumpItem('it_1', by: 'gril');
    expect(jsonDecode(posts['/api/items/it_1/bump']!), {'by': 'gril'});
    await c.pickTable('7');
    expect(posts.containsKey('/api/tables/7/pick'), isTrue);
    await expectLater(
      c.bumpItem('missing', by: 'x'),
      throwsA(isA<ApiException>().having((e) => e.message, 'message', 'položka neexistuje')),
    );

    c.dispose();
    await server.close(force: true);
  });
}
