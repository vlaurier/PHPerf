<?php

declare(strict_types=1);

fwrite(STDERR, "[demo] exécution du scénario de démonstration…\n");

require __DIR__ . '/FakeConnection.php';

use Doctrine\DBAL\FakeConnection;

$db = new FakeConnection();

// Anti-pattern N+1 : une requête par itération.
$orders = [];
for ($i = 0; $i < 50; $i++) {
    $orders[] = $db->query("SELECT * FROM orders WHERE customer_id = $i");
}

// Anti-pattern CPU : hachage lourd répété sur les mêmes données.
$checksums = [];
foreach ($orders as $order) {
    $payload = json_encode($order);
    for ($round = 0; $round < 200; $round++) {
        $payload = hash('sha256', $payload);
    }
    $checksums[$order['rows'][0]] = $payload;
}

// Anti-pattern mémoire/CPU : agrégation de tableaux en boucle.
$merged = [];
foreach ($orders as $order) {
    $merged = array_merge($merged, $order['rows']);
}

fwrite(STDERR, '[demo] terminé : ' . count($checksums) . " empreintes, " . count($merged) . " lignes fusionnées\n");
