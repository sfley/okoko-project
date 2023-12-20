<?php

class Conso extends CI_Model {

  function get_conso($year) {
    $result_stock_annuel = array();
    $result_stock_mois = array();
	$first_value = 0;
    $last_value = 0;
	$total_conso = 0;

    for ($month = 1; $month <= 12; $month++) {
        $monthFormatted = str_pad($month, 2, '0', STR_PAD_LEFT);
        $value_select_stock_month_Date = "{$year}-{$monthFormatted}-01";

        // trouver la première date du mois
        $query_first_date = $this->db->query(
            "SELECT MIN(DATE) AS first_date
             FROM `json_import` 
             WHERE DATE >= ? AND DATE <= LAST_DAY(?)
             AND heure < '00:01:00';",
            array($value_select_stock_month_Date, $value_select_stock_month_Date)
        );

        // trouver la dernière date du mois
        $query_last_date = $this->db->query(
            "SELECT MAX(DATE) AS last_date
             FROM `json_import` 
             WHERE DATE >= ? AND DATE <= LAST_DAY(?)
             AND heure < '00:01:00';",
            array($value_select_stock_month_Date, $value_select_stock_month_Date)
        );

        //  PE1_Fuellstand_kg pour les deux dates
        $first_date_result = $query_first_date->row();
        $last_date_result = $query_last_date->row();

        if (!$first_date_result || !$last_date_result) {
            
            continue;
        }

        $first_date = $first_date_result->first_date;
        $last_date = $last_date_result->last_date;

        $query_first_value = $this->db->query(
            "SELECT PE1_Fuellstand_kg AS first_value
             FROM `json_import` 
             WHERE DATE = ? AND heure < '00:01:00';",
            array($first_date)
        );

        $query_last_value = $this->db->query(
            "SELECT PE1_Fuellstand_kg AS last_value
             FROM `json_import` 
             WHERE DATE = ? AND heure < '00:01:00';",
            array($last_date)
        );

        // checher la valeur recharge (remplissage du silo) dans la base de données si $conso est négative
        $conso = 0;
        $total_recharge = 0;

        $first_value_result = $query_first_value->row();
        $last_value_result = $query_last_value->row();

        if ($first_value_result && $last_value_result) {
            $first_value = $first_value_result->first_value;
            $last_value = $last_value_result->last_value;

            $conso = $first_value - $last_value;

            if ($conso < 0) {
                $query_recharge = $this->db->query(
                    "SELECT COALESCE(SUM(CASE WHEN recharge IS NOT NULL THEN recharge END), 0) AS total_recharge
                     FROM `json_import` t
                     WHERE t.DATE >= ? AND t.DATE <= DATE_ADD(?, INTERVAL 7 DAY) AND t.heure < '00:01:00';",
                    array($value_select_stock_month_Date, $value_select_stock_month_Date)
                );

                $total_recharge_result = $query_recharge->row();

                if ($total_recharge_result) {
                    $total_recharge = $total_recharge_result->total_recharge;
                }
            }
        }

        // Mise en forme des noms des mois
        $monthName = "'" . date('F', mktime(0, 0, 0, $month, 1)) . "'";

$monthNamesFrench = array(
    'January' => 'janvier',
    'February' => 'février',
    'March' => 'mars',
    'April' => 'avril',
    'May' => 'mai',
    'June' => 'juin',
    'July' => 'juillet',
    'August' => 'août',
    'September' => 'septembre',
    'October' => 'octobre',
    'November' => 'novembre',
    'December' => 'décembre'
);

$monthName = "'" . $monthNamesFrench[date('F', mktime(0, 0, 0, $month, 1))] . "'";

        // Calcul de la conso
        $conso = ABS($first_value + $total_recharge) - $last_value;

       if ($conso != 0) {
    $result_stock_mois[] = array(
        'mois' => $monthName,
        'conso' => $conso ?? 0 // Use 0 if the value is still null
    );
	
	$total_conso += $conso;
		
	
}

        
    }
    
    $result_stock_annuel['mois'] = implode(array_column($result_stock_mois, 'mois'), ",");
    $result_stock_annuel['conso'] = implode(array_column($result_stock_mois, 'conso'), ",");
    $result_stock_annuel['name'] = 'Consommation totale ' . $year .' : '.$total_conso." kg";
    $result_stock_annuel['annee'] = $year;

  

    return $result_stock_annuel;
}


 



    function get_conso_day() {
        $result_stock_day = array();

        $query_stock_day = $this->db->query('SELECT Date, PE1_Fuellstand_kg as conso_day FROM `json_import` WHERE heure < "00:00:40" ORDER BY date');

        if ($query_stock_day->num_rows() > 0) {
            $stock = $query_stock_day->result_array();
            foreach ($stock as $row) {
                $result_stock_day_date[] = "'" . $row['Date'] . "'";
            }
            foreach ($stock as $row) {
                $result_stock_day_data[] = $row['conso_day'];
            }

            $result_stock_day['date'] = implode($result_stock_day_date, ",");
            $result_stock_day['data'] = implode($result_stock_day_data, ",");

            // var_dump($result_stock_day); // Dumping the result for debugging

            return $result_stock_day;
        }
    }

    function get_conso_now() {
		
		$result_stock_day_data = array();
        $result_stock_now = array();

        $query_stock_now = $this->db->query('SELECT PE1_Fuellstand_kg as conso_now FROM `json_import` ORDER BY date DESC LIMIT 1');

        if ($query_stock_now->num_rows() > 0) {
            $stock = $query_stock_now->result_array();
            foreach ($stock as $row) {
                $result_stock_day_data[] = $row['conso_now'];
            }

            $result_stock_now['data'] = implode($result_stock_day_data, ",");

            // var_dump($result_stock_now); // Dumping the result for debugging

            return $result_stock_now;
        }
    }
}
