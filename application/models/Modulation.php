<?php
Class modulation extends CI_Model {
	
function get() {

	
$value_select_modulation_1= 'CONCAT(DATE_FORMAT(Date,"%d/%m")," ",DATE_FORMAT(Heure,"%Hh%i"))';
$value_select_modulation_2=	'PE1_modulation';
$query_modulation = $this->db->query('SELECT '.$value_select_modulation_1.','.$value_select_modulation_2.' FROM json_import WHERE Date BETWEEN NOW() - INTERVAL 7 DAY AND NOW() AND MINUTE(heure) IN (0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55) LIMIT 4500  ');


if($query_modulation->num_rows() > 0) {	
	
$modulation = $query_modulation->result_array();
foreach($modulation as $row) {	$result_modulation_date[]="'".$row[$value_select_modulation_1]."'";}
foreach($modulation as $row) {$result_modulation_data[]=$row[$value_select_modulation_2];}

$result_modulation['date']=implode($result_modulation_date,",");
$result_modulation['data']=implode($result_modulation_data,",");
$result_modulation['data_name']=$value_select_modulation_2;

return $result_modulation;
}
}

}
	
